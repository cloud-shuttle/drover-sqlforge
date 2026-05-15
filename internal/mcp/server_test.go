package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServer_ServeHTTP(t *testing.T) {
	server := NewServer("test-api-key", nil, nil)

	tests := []struct {
		name           string
		method         string
		authHeader     string
		body           string
		expectedStatus int
		expectedCode   int // For JSON-RPC Error Code
	}{
		{
			name:           "Unauthorized - Missing Auth",
			method:         http.MethodPost,
			authHeader:     "",
			body:           `{}`,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Unauthorized - Invalid Auth",
			method:         http.MethodPost,
			authHeader:     "Bearer wrong-key",
			body:           `{}`,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Method Not Allowed",
			method:         http.MethodGet,
			authHeader:     "Bearer test-api-key",
			body:           ``,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "Invalid JSON Body",
			method:         http.MethodPost,
			authHeader:     "Bearer test-api-key",
			body:           `{invalid json`,
			expectedStatus: http.StatusOK,
			expectedCode:   ParseError,
		},
		{
			name:           "Invalid JSON-RPC Version",
			method:         http.MethodPost,
			authHeader:     "Bearer test-api-key",
			body:           `{"jsonrpc":"1.0","id":1,"method":"tools/list"}`,
			expectedStatus: http.StatusOK,
			expectedCode:   InvalidRequest,
		},
		{
			name:           "Method Not Found",
			method:         http.MethodPost,
			authHeader:     "Bearer test-api-key",
			body:           `{"jsonrpc":"2.0","id":1,"method":"unknown_method"}`,
			expectedStatus: http.StatusOK,
			expectedCode:   MethodNotFound,
		},
		{
			name:           "Tools List Success",
			method:         http.MethodPost,
			authHeader:     "Bearer test-api-key",
			body:           `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
			expectedStatus: http.StatusOK,
			expectedCode:   0, // success
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/mcp", bytes.NewBufferString(tt.body))
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			w := httptest.NewRecorder()
			server.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected HTTP status %d, got %d", tt.expectedStatus, w.Code)
			}

			if w.Code == http.StatusOK && tt.expectedCode != 0 {
				var resp Response
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to parse JSON-RPC error response: %v", err)
				}
				if resp.Error == nil {
					t.Fatalf("Expected JSON-RPC error, got nil")
				}
				if resp.Error.Code != tt.expectedCode {
					t.Errorf("Expected JSON-RPC error code %d, got %d", tt.expectedCode, resp.Error.Code)
				}
			} else if w.Code == http.StatusOK && tt.expectedCode == 0 {
				var resp Response
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to parse JSON-RPC success response: %v", err)
				}
				if resp.Error != nil {
					t.Errorf("Expected success, but got error: %v", resp.Error)
				}
			}
		})
	}
}

func TestServer_ServeHTTP_NoAuth(t *testing.T) {
	// If API key is empty, it shouldn't check auth
	server := NewServer("", nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected HTTP status 200 without auth, got %d", w.Code)
	}
}

func TestServer_ServeHTTP_ToolError(t *testing.T) {
	server := NewServer("", nil, nil)
	// Add a dummy tool that fails
	server.Registry.Register(Tool{
		Name: "fail_tool",
		Handler: func(ctx context.Context, params []byte) (interface{}, error) {
			return nil, fmt.Errorf("tool failed")
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"fail_tool"}`))
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse JSON-RPC error response: %v", err)
	}
	if resp.Error == nil {
		t.Fatalf("Expected JSON-RPC error, got nil")
	}
	if resp.Error.Code != InternalError {
		t.Errorf("Expected JSON-RPC error code %d, got %d", InternalError, resp.Error.Code)
	}
}
