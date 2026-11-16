package mcp

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/drover-org/drover-sqlforge/internal/project"
)

// DefaultMaxBodyBytes caps a single JSON-RPC request body.
const DefaultMaxBodyBytes int64 = 1 << 20 // 1 MiB

type Server struct {
	Registry *Registry
	APIKey   string
	Runtime  *project.Runtime
	Plans    *PlanStore
	// MaxBodyBytes caps the request body; zero means DefaultMaxBodyBytes.
	MaxBodyBytes int64
}

func NewServer(apiKey string, rt *project.Runtime) *Server {
	plans := NewPlanStore()
	registry := NewRegistry()
	registry.InitializeCoreTools(rt, plans)

	return &Server{
		Registry: registry,
		APIKey:   apiKey,
		Runtime:  rt,
		Plans:    plans,
	}
}

// ServeHTTP implements the JSON-RPC 2.0 handler for the MCP endpoint
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Auth check if an API key is configured. ValidateBind refuses to start a
	// non-loopback server without one.
	if s.APIKey != "" && !bearerMatches(r.Header.Get("Authorization"), s.APIKey) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := s.MaxBodyBytes
	if limit <= 0 {
		limit = DefaultMaxBodyBytes
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		sendError(w, nil, ParseError, "Parse error")
		return
	}

	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		sendError(w, nil, ParseError, "Parse error")
		return
	}

	if req.JSONRPC != "2.0" {
		sendError(w, req.ID, InvalidRequest, "Invalid JSON-RPC version")
		return
	}

	// Dispatch to the correct tool
	tool, ok := s.Registry.Get(req.Method)
	if !ok {
		// Special case: exposing tools list natively for agents
		if req.Method == "tools/list" {
			sendSuccess(w, req.ID, s.Registry.ListTools())
			return
		}

		sendError(w, req.ID, MethodNotFound, "Method not found")
		return
	}

	result, err := tool.Handler(r.Context(), req.Params)
	if err != nil {
		sendError(w, req.ID, InternalError, err.Error())
		return
	}

	sendSuccess(w, req.ID, result)
}

// bearerMatches compares an Authorization header against the expected key in
// constant time.
func bearerMatches(header, key string) bool {
	token, ok := strings.CutPrefix(header, "Bearer ")
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(key)) == 1
}

// ValidateBind refuses configurations that would expose the MCP tools
// (including apply_change, which writes to the warehouse) on a non-loopback
// interface without authentication.
func ValidateBind(host, apiKey string) error {
	if apiKey != "" || isLoopback(host) {
		return nil
	}
	return fmt.Errorf("refusing to bind MCP server to %q without an API key: set SQLFORGE_MCP_API_KEY (or --api-key), or bind to 127.0.0.1", host)
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func sendSuccess(w http.ResponseWriter, id json.RawMessage, result interface{}) {
	resp := NewResponse(id, result)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func sendError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	resp := NewErrorResponse(id, code, message, nil)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
	log.Printf("MCP Error: %s (Code: %d)", message, code)
}
