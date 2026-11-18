package mcp

import "os"

// Governance is the optional identity, hook, and audit configuration.
type Governance struct {
	AuthMode    string
	TokenHeader string
	Verifier    TokenVerifier
	HookNames   []string
	Audit       *AuditLog
}

// UseGovernance installs OIDC verification, the hook chain, and the audit sink.
// The default chain always redacts get_model SQL for agents. A warden hook is
// accepted only when this binary can construct one, which the public module
// cannot do without a private dependency.
func (s *Server) UseGovernance(g Governance) error {
	if g.AuthMode != "" {
		s.AuthMode = g.AuthMode
	}
	s.TokenHeader = g.TokenHeader
	s.Verifier = g.Verifier
	if g.AuthMode == "oidc" && g.Verifier == nil {
		return errOIDCVerifier
	}
	hooks := []ToolHook{redactHook{}}
	for _, name := range g.HookNames {
		hook, err := hookByName(name)
		if err != nil {
			return err
		}
		if hook != nil {
			hooks = append(hooks, hook)
		}
	}
	s.Hooks = &HookChain{hooks: hooks}
	if g.Audit != nil {
		s.Audit = g.Audit
	}
	return nil
}

var errOIDCVerifier = errString("oidc mode requires a verifier")

type errString string

func (e errString) Error() string { return string(e) }

func hookByName(name string) (ToolHook, error) {
	switch name {
	case "", "redact":
		return nil, nil
	case "warden":
		return newWardenHook()
	default:
		return nil, errString("unknown mcp hook " + name)
	}
}

func defaultAudit() *AuditLog {
	return NewAuditLog(os.Stderr)
}
