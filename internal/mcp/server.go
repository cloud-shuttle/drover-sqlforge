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

	"github.com/cloud-shuttle/drover-sqlforge/internal/project"
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
	// ReadOnly is set when write tools have been removed from the registry.
	ReadOnly bool
	// AllowProtectedApply permits apply_change against protected environments.
	AllowProtectedApply bool
	// Protected is the set of environment names that refuse apply_change.
	// Nil means the default (prod). An empty non-nil map protects nothing.
	Protected map[string]struct{}
	// AuthMode is api_key (default) or oidc. OIDC never falls back to the API key.
	AuthMode string
	// TokenHeader is the header Gateway uses to forward an OIDC token.
	TokenHeader string
	Verifier    TokenVerifier
	Hooks       *HookChain
	Audit       *AuditLog
}

// ServerConfig is the optional MCP policy applied after NewServer.
type ServerConfig struct {
	ReadOnly            bool
	AllowProtectedApply bool
	Protected           map[string]struct{}
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
		Hooks:    &HookChain{hooks: []ToolHook{redactHook{}}},
		Audit:    defaultAudit(),
	}
}

// Configure applies read-only mode and the protected-environment policy.
// Read-only removes write tools, so tools/list and dispatch both omit them.
func (s *Server) Configure(cfg ServerConfig) {
	s.AllowProtectedApply = cfg.AllowProtectedApply
	s.Protected = cfg.Protected
	if cfg.ReadOnly {
		s.UseReadOnly()
	}
}

// UseReadOnly unregisters every tool that writes to the project or warehouse.
func (s *Server) UseReadOnly() {
	s.ReadOnly = true
	if s.Registry != nil {
		s.Registry.Unregister(writeToolNames...)
	}
}

// ServeHTTP implements the JSON-RPC 2.0 handler for the MCP endpoint.
// Order is auth, pre-tool hook, tool, post-tool hook, then the audit line.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorize(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	req, ok := readJSONRPC(w, r, s.MaxBodyBytes)
	if !ok {
		return
	}
	s.dispatch(w, r, req, actor)
}

func readJSONRPC(w http.ResponseWriter, r *http.Request, maxBody int64) (Request, bool) {
	limit := maxBody
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
			return Request{}, false
		}
		sendError(w, nil, ParseError, "Parse error")
		return Request{}, false
	}
	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		sendError(w, nil, ParseError, "Parse error")
		return Request{}, false
	}
	if req.JSONRPC != "2.0" {
		sendError(w, req.ID, InvalidRequest, "Invalid JSON-RPC version")
		return Request{}, false
	}
	return req, true
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
	return ValidateBindMode(host, apiKey, false)
}

// ValidateBindMode allows a non-loopback bind when an API key or OIDC mode is set.
func ValidateBindMode(host, apiKey string, oidc bool) error {
	if apiKey != "" || oidc || isLoopback(host) {
		return nil
	}
	return fmt.Errorf("refusing to bind MCP server to %q without an API key or oidc auth: set SQLFORGE_MCP_API_KEY (or --api-key), configure mcp.auth.mode oidc, or bind to 127.0.0.1", host)
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
	// The message can echo SQL. The audit log records the class; this line records the code only.
	log.Printf("MCP error code %d", code)
}
