package mcp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBearerMatches(t *testing.T) {
	cases := []struct {
		header string
		want   bool
	}{
		{"Bearer secret", true},
		{"Bearer secret2", false},
		{"Bearer secre", false},
		{"bearer secret", false},
		{"secret", false},
		{"Bearer ", false},
		{"", false},
		{"Basic secret", false},
	}
	for _, c := range cases {
		if got := bearerMatches(c.header, "secret"); got != c.want {
			t.Errorf("bearerMatches(%q) = %v, want %v", c.header, got, c.want)
		}
	}
}

func TestValidateBind(t *testing.T) {
	cases := []struct {
		host, key string
		wantErr   bool
	}{
		{"127.0.0.1", "", false},
		{"localhost", "", false},
		{"::1", "", false},
		{"0.0.0.0", "", true},
		{"10.0.0.5", "", true},
		{"", "", true},
		{"0.0.0.0", "k", false},
		{"10.0.0.5", "k", false},
	}
	for _, c := range cases {
		err := ValidateBind(c.host, c.key)
		if (err != nil) != c.wantErr {
			t.Errorf("ValidateBind(%q, key=%v) err = %v, wantErr %v", c.host, c.key != "", err, c.wantErr)
		}
		if err != nil && strings.Contains(err.Error(), c.key) && c.key != "" {
			t.Errorf("error leaks key: %v", err)
		}
	}
}

func TestServer_RejectsOversizeBody(t *testing.T) {
	s := NewServer("k", nil)
	s.MaxBodyBytes = 64
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":"` + strings.Repeat("a", 200) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer k")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
}

func TestServer_DefaultBodyLimitAllowsNormalRequests(t *testing.T) {
	s := NewServer("k", nil)
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer k")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"result"`) {
		t.Fatalf("expected JSON-RPC result, got %s", rec.Body.String())
	}
}

func TestServer_AuthCheckedBeforeBodyRead(t *testing.T) {
	s := NewServer("k", nil)
	s.MaxBodyBytes = 8
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(strings.Repeat("x", 100)))
	req.Header.Set("Authorization", "Bearer wrong")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
