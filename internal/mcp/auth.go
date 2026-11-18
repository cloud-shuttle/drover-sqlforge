package mcp

import (
	"errors"
	"net/http"
	"strings"
)

const defaultOIDCHeader = "x-drover-identity-token"

// TokenVerifier checks an OIDC token and returns the actor it proves.
type TokenVerifier interface {
	Verify(token string) (Actor, error)
}

// AuthError is a stable class for an authentication failure.
// Error text is the class only, so logs cannot echo a token.
type AuthError struct {
	Class string
}

func (e *AuthError) Error() string {
	if e == nil || e.Class == "" {
		return "oidc bad_signature"
	}
	return "oidc " + e.Class
}

func (s *Server) authorize(w http.ResponseWriter, r *http.Request) (Actor, bool) {
	if s.AuthMode == "oidc" {
		return s.authorizeOIDC(w, r)
	}
	if s.APIKey != "" && !bearerMatches(r.Header.Get("Authorization"), s.APIKey) {
		s.recordAuthFailure(r, "bad_api_key")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return Actor{}, false
	}
	if s.APIKey != "" {
		return Actor{Subject: "api-key", Via: "api_key"}, true
	}
	return Actor{Subject: "local", Via: "local"}, true
}

func (s *Server) authorizeOIDC(w http.ResponseWriter, r *http.Request) (Actor, bool) {
	actor, class, err := s.verifyOIDC(r)
	if err != nil {
		s.recordAuthFailure(r, class)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return Actor{}, false
	}
	return actor, true
}

func (s *Server) verifyOIDC(r *http.Request) (Actor, string, error) {
	if s.Verifier == nil {
		return Actor{}, "bad_signature", &AuthError{Class: "bad_signature"}
	}
	header := s.TokenHeader
	if header == "" {
		header = defaultOIDCHeader
	}
	raw := strings.TrimSpace(r.Header.Get(header))
	if raw == "" {
		return Actor{}, "missing_token", &AuthError{Class: "missing_token"}
	}
	actor, err := s.Verifier.Verify(raw)
	if err != nil {
		return Actor{}, authClass(err), err
	}
	actor.Via = "oidc"
	return actor, "", nil
}

func authClass(err error) string {
	var ae *AuthError
	if errors.As(err, &ae) && ae.Class != "" {
		return ae.Class
	}
	return "bad_signature"
}

func (s *Server) recordAuthFailure(r *http.Request, class string) {
	line := s.baseRecord(r, Actor{}, "auth")
	line.Outcome = "auth_failed"
	line.ErrorClass = safeLog(class, 64)
	s.writeAudit(line)
}
