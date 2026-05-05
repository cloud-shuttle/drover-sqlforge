package mcp

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/drover-org/drover-sqlforge/internal/graph"
	"github.com/drover-org/drover-sqlforge/internal/semantic"
)

// FuzzServerHTTP tests the JSON-RPC HTTP boundary to ensure
// that malformed payloads do not panic the server.
func FuzzServerHTTP(f *testing.F) {
	// Seed corpus with valid and invalid requests
	f.Add([]byte(`{"jsonrpc": "2.0", "method": "tools/list", "id": 1}`))
	f.Add([]byte(`{"jsonrpc": "1.0", "method": "tools/list", "id": 1}`))
	f.Add([]byte(`{malformed json}`))
	f.Add([]byte(``))

	dag := graph.NewDAG()
	semGraph := &semantic.Graph{}
	server := NewServer("", dag, semGraph)

	f.Fuzz(func(t *testing.T, payload []byte) {
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(payload))
		w := httptest.NewRecorder()

		// Server should never panic, regardless of input
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Server panicked on payload %q: %v", payload, r)
			}
		}()

		server.ServeHTTP(w, req)
	})
}
