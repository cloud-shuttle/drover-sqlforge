package mcp

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// KeySet maps a JWT kid to a public key and can refresh once on an unknown kid.
type KeySet struct {
	mu        sync.Mutex
	keys      map[string]any
	refresh   func() (map[string]any, error)
	refreshed bool
}

func NewKeySet(keys map[string]any, refresh func() (map[string]any, error)) *KeySet {
	copied := make(map[string]any, len(keys))
	for kid, key := range keys {
		copied[kid] = key
	}
	return &KeySet{keys: copied, refresh: refresh}
}

func (k *KeySet) Get(tokenKid, alg string) (any, error) {
	if alg != "RS256" && alg != "ES256" {
		return nil, fmt.Errorf("rejected signing method")
	}
	if key, ok := k.lookup(tokenKid); ok {
		return key, nil
	}
	if err := k.refreshOnce(); err != nil {
		return nil, err
	}
	if key, ok := k.lookup(tokenKid); ok {
		return key, nil
	}
	return nil, errUnknownKID
}

func (k *KeySet) lookup(kid string) (any, bool) {
	if k == nil || kid == "" {
		return nil, false
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	key, ok := k.keys[kid]
	return key, ok
}

func (k *KeySet) refreshOnce() error {
	if k == nil || k.refresh == nil {
		return errUnknownKID
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.refreshed {
		return nil
	}
	k.refreshed = true
	keys, err := k.refresh()
	if err != nil {
		return err
	}
	if k.keys == nil {
		k.keys = map[string]any{}
	}
	for kid, key := range keys {
		k.keys[kid] = key
	}
	return nil
}

var errUnknownKID = fmt.Errorf("unknown kid")

func allowedIssuerURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid url")
	}
	host := parsed.Hostname()
	if parsed.Scheme == "https" {
		return nil
	}
	if parsed.Scheme == "http" && (host == "localhost" || host == "127.0.0.1" || host == "::1") {
		return nil
	}
	return fmt.Errorf("jwks url must be https or loopback")
}

func fetchJSON(ctx context.Context, client *http.Client, rawURL string, dest any) error {
	if err := allowedIssuerURL(rawURL); err != nil {
		return err
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("jwks fetch failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks http %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(dest)
}

// DiscoverJWKS loads the issuer's published keys.
func DiscoverJWKS(ctx context.Context, issuer string, client *http.Client) (map[string]any, error) {
	var doc struct {
		JWKSURI string `json:"jwks_uri"`
	}
	wellKnown := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	if err := fetchJSON(ctx, client, wellKnown, &doc); err != nil {
		return nil, err
	}
	if doc.JWKSURI == "" {
		return nil, fmt.Errorf("openid configuration has no jwks_uri")
	}
	var raw json.RawMessage
	if err := fetchJSON(ctx, client, doc.JWKSURI, &raw); err != nil {
		return nil, err
	}
	return ParseJWKS(raw)
}

type jwkKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

// ParseJWKS parses an RSA or P-256 EC key set.
func ParseJWKS(data []byte) (map[string]any, error) {
	var doc struct {
		Keys []jwkKey `json:"keys"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	out := make(map[string]any, len(doc.Keys))
	for _, key := range doc.Keys {
		pub, err := key.publicKey()
		if err != nil {
			return nil, err
		}
		if key.Kid == "" {
			return nil, fmt.Errorf("jwks key missing kid")
		}
		out[key.Kid] = pub
	}
	return out, nil
}

func (k jwkKey) publicKey() (any, error) {
	switch k.Kty {
	case "RSA":
		return k.rsa()
	case "EC":
		return k.ec()
	default:
		return nil, fmt.Errorf("unsupported jwk kty")
	}
}

func (k jwkKey) rsa() (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("invalid jwk modulus")
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("invalid jwk exponent")
	}
	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}
	if e == 0 {
		return nil, fmt.Errorf("invalid jwk exponent")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
}

func (k jwkKey) ec() (*ecdsa.PublicKey, error) {
	if k.Crv != "P-256" {
		return nil, fmt.Errorf("unsupported jwk curve")
	}
	x, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, fmt.Errorf("invalid jwk x")
	}
	y, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, fmt.Errorf("invalid jwk y")
	}
	return &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(x),
		Y:     new(big.Int).SetBytes(y),
	}, nil
}
