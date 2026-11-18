package mcp

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestOIDCVerifierTable(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	const issuer = "https://issuer.example"
	const audience = "sqlforge-mcp"
	verifier := &OIDCVerifier{
		Issuer:   issuer,
		Audience: audience,
		Keys: NewKeySet(map[string]any{
			"kid-1":  &rsaKey.PublicKey,
			"kid-ec": &ecKey.PublicKey,
		}, nil),
		Now: func() time.Time { return now },
	}

	good := signRSA(t, rsaKey, "kid-1", oidcClaims(issuer, audience, "user-1", now.Unix(), now.Add(time.Hour).Unix()))
	actor, err := verifier.Verify(good)
	if err != nil {
		t.Fatal(err)
	}
	if actor.Subject != "user-1" || actor.Kind != "member" || actor.OrgID != "org-1" || actor.Via != "oidc" {
		t.Fatalf("actor = %+v", actor)
	}

	ecToken := signEC(t, ecKey, "kid-ec", oidcClaims(issuer, audience, "ec-user", now.Unix(), now.Add(time.Hour).Unix()))
	if _, err := verifier.Verify(ecToken); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		token string
		class string
		kid   bool
	}{
		{"wrong iss", signRSA(t, rsaKey, "kid-1", oidcClaims("https://other.example", audience, "user-1", now.Unix(), now.Add(time.Hour).Unix())), "wrong_issuer", false},
		{"wrong aud", signRSA(t, rsaKey, "kid-1", oidcClaims(issuer, "other-api", "user-1", now.Unix(), now.Add(time.Hour).Unix())), "wrong_audience", false},
		{"expired", signRSA(t, rsaKey, "kid-1", oidcClaims(issuer, audience, "user-1", now.Add(-2*time.Hour).Unix(), now.Add(-2*time.Minute).Unix())), "expired", false},
		{"none", noneToken(issuer, audience, now), "bad_signature", false},
		{"hs256", signHS(t, oidcClaims(issuer, audience, "user-1", now.Unix(), now.Add(time.Hour).Unix())), "bad_signature", false},
		{"unknown kid", signRSA(t, rsaKey, "missing", oidcClaims(issuer, audience, "user-1", now.Unix(), now.Add(time.Hour).Unix())), "bad_signature", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := verifier.Verify(tc.token)
			var ae *AuthError
			if !errors.As(err, &ae) || ae.Class != tc.class {
				t.Fatalf("err = %v, want class %s", err, tc.class)
			}
			if tc.kid && !errors.Is(err, errUnknownKID) {
				t.Fatalf("err = %v, want unknown kid", err)
			}
			if tc.token != "" && errors.As(err, &ae) && ae.Error() != "oidc "+tc.class && !tc.kid {
				t.Fatalf("error text = %q", ae.Error())
			}
		})
	}

	skew := signRSA(t, rsaKey, "kid-1", oidcClaims(issuer, audience, "user-1", now.Unix(), now.Add(-30*time.Second).Unix()))
	if _, err := verifier.Verify(skew); err != nil {
		t.Fatalf("30s skew should pass: %v", err)
	}
}

func TestOIDCRequiredOrg(t *testing.T) {
	now := time.Now()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	verifier := &OIDCVerifier{
		Issuer: "https://issuer.example", Audience: "sqlforge-mcp", RequiredOrg: "org-1",
		Keys: NewKeySet(map[string]any{"kid-1": &key.PublicKey}, nil),
		Now:  func() time.Time { return now },
	}
	claims := oidcClaims(verifier.Issuer, verifier.Audience, "user-1", now.Unix(), now.Add(time.Hour).Unix())
	claims["urn:zitadel:iam:org:id"] = "org-2"
	_, err = verifier.Verify(signRSA(t, key, "kid-1", claims))
	var ae *AuthError
	if !errors.As(err, &ae) || ae.Class != "wrong_org" {
		t.Fatal(err)
	}
}

func TestOIDCRefreshesUnknownKidOnce(t *testing.T) {
	now := time.Now()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	verifier := &OIDCVerifier{
		Issuer: "https://issuer.example", Audience: "sqlforge-mcp",
		Now: func() time.Time { return now },
		Keys: NewKeySet(nil, func() (map[string]any, error) {
			calls++
			return map[string]any{"kid-1": &key.PublicKey}, nil
		}),
	}
	token := signRSA(t, key, "kid-1", oidcClaims(verifier.Issuer, verifier.Audience, "user-1", now.Unix(), now.Add(time.Hour).Unix()))
	if _, err := verifier.Verify(token); err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Verify(token); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("refresh calls = %d", calls)
	}
}

func TestLocalJWKSFixture(t *testing.T) {
	now := time.Now()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	fixture := jwksFixture("kid-1", &key.PublicKey)
	keys, err := ParseJWKS(fixture)
	if err != nil {
		t.Fatal(err)
	}
	verifier := &OIDCVerifier{
		Issuer: "https://issuer.example", Audience: "sqlforge-mcp",
		Keys: NewKeySet(keys, nil),
		Now:  func() time.Time { return now },
	}
	token := signRSA(t, key, "kid-1", oidcClaims(verifier.Issuer, verifier.Audience, "user-1", now.Unix(), now.Add(time.Hour).Unix()))
	if _, err := verifier.Verify(token); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverJWKS(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	fixture := jwksFixture("kid-1", &key.PublicKey)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			fmt.Fprintf(w, `{"jwks_uri":"%s/jwks"}`, srv.URL)
		case "/jwks":
			_, _ = w.Write(fixture)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	keys, err := DiscoverJWKS(context.Background(), srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := keys["kid-1"].(*rsa.PublicKey); !ok {
		t.Fatalf("keys = %#v", keys)
	}
	if err := allowedIssuerURL("http://example.com/jwks"); err == nil {
		t.Fatal("public http jwks url should be rejected")
	}
}

func oidcClaims(issuer, audience, sub string, iat, exp int64) jwt.MapClaims {
	return jwt.MapClaims{
		"iss":                               issuer,
		"aud":                               audience,
		"sub":                               sub,
		"iat":                               float64(iat),
		"nbf":                               float64(iat),
		"exp":                               float64(exp),
		"kind":                              "member",
		"urn:zitadel:iam:org:id":            "org-1",
		"urn:zitadel:iam:org:project:roles": map[string]any{"reader": map[string]any{}},
	}
}

func signRSA(t *testing.T, key *rsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func signEC(t *testing.T, key *ecdsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func signHS(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func noneToken(issuer, audience string, now time.Time) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload, _ := json.Marshal(oidcClaims(issuer, audience, "user-1", now.Unix(), now.Add(time.Hour).Unix()))
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + "."
}

func jwksFixture(kid string, pub *rsa.PublicKey) []byte {
	raw, _ := json.Marshal(map[string]any{
		"keys": []map[string]string{{
			"kty": "RSA",
			"kid": kid,
			"alg": "RS256",
			"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
		}},
	})
	return raw
}

func TestParseECJWKS(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{
		"keys": []map[string]string{{
			"kty": "EC",
			"kid": "ec",
			"crv": "P-256",
			"x":   base64.RawURLEncoding.EncodeToString(fixedCoord(key.X)),
			"y":   base64.RawURLEncoding.EncodeToString(fixedCoord(key.Y)),
		}},
	})
	keys, err := ParseJWKS(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := keys["ec"].(*ecdsa.PublicKey); !ok {
		t.Fatalf("key = %#v", keys["ec"])
	}
	if _, err := ParseJWKS([]byte(`{"keys":[{"kty":"oct","kid":"k"}]}`)); err == nil {
		t.Fatal("expected unsupported kty")
	}
	if _, err := ParseJWKS([]byte(`{"keys":[{"kty":"RSA","n":"AQ","e":"AQ"}]}`)); err == nil {
		t.Fatal("expected missing kid")
	}
}

func fixedCoord(n *big.Int) []byte {
	b := n.Bytes()
	out := make([]byte, 32)
	copy(out[32-len(b):], b)
	return out
}

func TestNewOIDCVerifierRequiresIssuer(t *testing.T) {
	if _, err := NewOIDCVerifier("", "aud", "", nil); err == nil {
		t.Fatal("expected error")
	}
	if err := ValidateBindMode("0.0.0.0", "", true); err != nil {
		t.Fatal(err)
	}
}
