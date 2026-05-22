package mcp

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/drover-org/drover-sqlforge/internal/project"
)

type Server struct {
	Registry *Registry
	APIKey   string
	Runtime  *project.Runtime
	Plans    *PlanStore
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
	// Simple Auth check if API key is configured
	if s.APIKey != "" {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer "+s.APIKey {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		sendError(w, nil, ParseError, "Parse error")
		return
	}
	defer r.Body.Close()

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
