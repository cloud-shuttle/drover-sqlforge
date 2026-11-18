package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// OIDCVerifier checks a Zitadel token forwarded by Drover Gateway.
// iss and aud are exact, and identity is never taken from convenience headers.
type OIDCVerifier struct {
	Issuer      string
	Audience    string
	RequiredOrg string
	Keys        *KeySet
	Now         func() time.Time
}

func NewOIDCVerifier(issuer, audience, requiredOrg string, client *http.Client) (*OIDCVerifier, error) {
	if issuer == "" || audience == "" {
		return nil, fmt.Errorf("oidc mode requires mcp.auth.issuer and mcp.auth.audience")
	}
	verifier := &OIDCVerifier{
		Issuer:      issuer,
		Audience:    audience,
		RequiredOrg: requiredOrg,
		Now:         time.Now,
	}
	verifier.Keys = NewKeySet(nil, func() (map[string]any, error) {
		return DiscoverJWKS(context.Background(), issuer, client)
	})
	return verifier, nil
}

func (v *OIDCVerifier) Verify(token string) (Actor, error) {
	if v == nil {
		return Actor{}, &AuthError{Class: "bad_signature"}
	}
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, v.keyfunc, v.options()...)
	if err != nil {
		return Actor{}, jwtFailure(err)
	}
	if parsed == nil || !parsed.Valid {
		return Actor{}, &AuthError{Class: "bad_signature"}
	}
	if err := v.checkOrg(claims); err != nil {
		return Actor{}, err
	}
	return actorFromClaims(claims), nil
}

func (v *OIDCVerifier) keyfunc(token *jwt.Token) (any, error) {
	if v == nil || v.Keys == nil || token == nil {
		return nil, errUnknownKID
	}
	kid, _ := token.Header["kid"].(string)
	alg, _ := token.Header["alg"].(string)
	return v.Keys.Get(kid, alg)
}

func (v *OIDCVerifier) options() []jwt.ParserOption {
	now := time.Now
	if v.Now != nil {
		now = v.Now
	}
	return []jwt.ParserOption{
		jwt.WithValidMethods([]string{"RS256", "ES256"}),
		jwt.WithIssuer(v.Issuer),
		jwt.WithAudience(v.Audience),
		jwt.WithLeeway(60 * time.Second),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(now),
	}
}

func (v *OIDCVerifier) checkOrg(claims jwt.MapClaims) error {
	if v.RequiredOrg == "" {
		return nil
	}
	org, _ := claims["urn:zitadel:iam:org:id"].(string)
	if org != v.RequiredOrg {
		return &AuthError{Class: "wrong_org"}
	}
	return nil
}

func jwtFailure(err error) error {
	class := "bad_signature"
	switch {
	case errors.Is(err, jwt.ErrTokenExpired):
		class = "expired"
	case errors.Is(err, jwt.ErrTokenInvalidAudience):
		class = "wrong_audience"
	case errors.Is(err, jwt.ErrTokenInvalidIssuer):
		class = "wrong_issuer"
	}
	if errors.Is(err, errUnknownKID) {
		return fmt.Errorf("%w: %w", &AuthError{Class: class}, errUnknownKID)
	}
	return &AuthError{Class: class}
}

func actorFromClaims(claims jwt.MapClaims) Actor {
	sub, _ := claims["sub"].(string)
	org, _ := claims["urn:zitadel:iam:org:id"].(string)
	kind := "agent"
	if raw, ok := claims["kind"].(string); ok && knownKind(raw) {
		kind = raw
	}
	return Actor{Subject: sub, OrgID: org, Kind: kind, Roles: roleNames(claims), Via: "oidc"}
}

func knownKind(kind string) bool {
	return kind == "member" || kind == "agent" || kind == "service"
}

func roleNames(claims jwt.MapClaims) []string {
	raw, ok := claims["urn:zitadel:iam:org:project:roles"].(map[string]any)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(raw))
	for name := range raw {
		if safeLog(name, 64) == "" {
			continue
		}
		names = append(names, name)
	}
	return names
}
