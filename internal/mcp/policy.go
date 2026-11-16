package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/drover-org/drover-sqlforge/internal/project"
)

// Options controls which MCP tools are exposed and where agents may write.
type Options struct {
	// ReadOnly registers only tools that never create plans or write to the
	// warehouse (list_models, get_model, list_metrics, query_metric,
	// list_snapshots).
	ReadOnly bool
	// ProtectedEnvironments are refused by apply_change unless
	// AllowProtectedApply is set. Matching is case-insensitive because
	// unquoted schema names fold case on most warehouses.
	ProtectedEnvironments []string
	// AllowProtectedApply lets apply_change run against a protected
	// environment. It is an explicit operator opt-in (--allow-protected-apply).
	AllowProtectedApply bool
}

// NewServerWithOptions builds a server and applies the tool policy in opts.
func NewServerWithOptions(apiKey string, rt *project.Runtime, opts Options) *Server {
	s := NewServer(apiKey, rt)
	s.Registry.ApplyOptions(rt, opts)
	return s
}

// ApplyOptions enforces opts on an initialised registry: in read-only mode
// mutating tools are removed, otherwise apply_change is wrapped with the
// protected-environment check.
func (r *Registry) ApplyOptions(rt *project.Runtime, opts Options) {
	if opts.ReadOnly {
		for _, name := range r.mutatingToolNames() {
			r.Unregister(name)
		}
		return
	}

	apply, ok := r.Get("apply_change")
	if !ok || opts.AllowProtectedApply {
		return
	}
	inner := apply.Handler
	protected := append([]string(nil), opts.ProtectedEnvironments...)
	apply.Handler = func(ctx context.Context, params []byte) (interface{}, error) {
		env := runtimeEnvName(rt)
		if IsProtectedEnvironment(env, protected) {
			return nil, fmt.Errorf("environment %q is protected: apply_change is disabled (restart the MCP server with --allow-protected-apply, or apply through a reviewed CLI run)", env)
		}
		return inner(ctx, params)
	}
	r.Register(apply)
}

// IsProtectedEnvironment reports whether env is in the protected list.
func IsProtectedEnvironment(env string, protected []string) bool {
	if env == "" {
		return false
	}
	for _, p := range protected {
		if strings.EqualFold(strings.TrimSpace(p), env) {
			return true
		}
	}
	return false
}

func (r *Registry) mutatingToolNames() []string {
	var names []string
	for name, t := range r.tools {
		if t.Mutating {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
