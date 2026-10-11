package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Supabase session tokens are ES256 JWTs; the public keys are published as a
// JWKS (auth/v1/.well-known/jwks.json). Ranked matches only trust the user id
// in a token whose signature, issuer, audience and expiry all check out.
const (
	jwksRefresh  = time.Hour
	jwksMinFetch = time.Minute // unknown kid: refetch at most this often
	authAudience = "authenticated"
	maxJWKSBytes = 64 << 10
)

var errBadToken = errors.New("auth: invalid token")

type verifier struct {
	jwksURL string
	issuer  string
	client  *http.Client

	mu      sync.Mutex
	keys    map[string]*ecdsa.PublicKey
	fetched time.Time
}

func newVerifier(jwksURL, issuer string) *verifier {
	return &verifier{jwksURL: jwksURL, issuer: issuer, client: &http.Client{Timeout: 5 * time.Second}, keys: map[string]*ecdsa.PublicKey{}}
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
}

type jwtClaims struct {
	Sub  string `json:"sub"`
	Meta struct {
		FullName string `json:"full_name"`
		Name     string `json:"name"`
		Custom   struct {
			GlobalName string `json:"global_name"`
		} `json:"custom_claims"`
	} `json:"user_metadata"`
	Iss string          `json:"iss"`
	Aud json.RawMessage `json:"aud"`
	Exp int64           `json:"exp"`
}

// account is a verified login: user id (sub) and its display name.
type account struct{ id, name string }

// verify returns the account of a valid token. The name follows the client
// (auth/session.ts): Discord global name, then full name, then name.
func (v *verifier) verify(token string) (account, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return account{}, errBadToken
	}
	var h jwtHeader
	if decodePart(parts[0], &h) != nil || h.Alg != "ES256" {
		return account{}, errBadToken
	}
	key, err := v.key(h.Kid)
	if err != nil {
		return account{}, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(sig) != 64 {
		return account{}, errBadToken
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if !ecdsa.Verify(key, sum[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
		return account{}, errBadToken
	}
	var c jwtClaims
	if decodePart(parts[1], &c) != nil || !v.claimsOK(c) {
		return account{}, errBadToken
	}
	name := c.Meta.Custom.GlobalName
	for _, n := range []string{c.Meta.FullName, c.Meta.Name} {
		if name == "" {
			name = n
		}
	}
	return account{id: c.Sub, name: cleanName(name)}, nil
}

func (v *verifier) claimsOK(c jwtClaims) bool {
	return c.Sub != "" && c.Iss == v.issuer && time.Now().Unix() < c.Exp && hasAudience(c.Aud)
}

// hasAudience accepts aud as a string or a list of strings.
func hasAudience(raw json.RawMessage) bool {
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return one == authAudience
	}
	var many []string
	if json.Unmarshal(raw, &many) != nil {
		return false
	}
	for _, a := range many {
		if a == authAudience {
			return true
		}
	}
	return false
}

func decodePart(s string, dst any) error {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}

// key returns the public key for kid, refreshing the JWKS when stale or when
// the kid is unknown (key rotation), at most once per jwksMinFetch.
func (v *verifier) key(kid string) (*ecdsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	k, ok := v.keys[kid]
	age := time.Since(v.fetched)
	if ok && age < jwksRefresh {
		return k, nil
	}
	if !ok && age < jwksMinFetch && !v.fetched.IsZero() {
		return nil, errBadToken
	}
	if err := v.fetchLocked(); err != nil && !ok {
		return nil, err
	}
	if k, ok = v.keys[kid]; !ok {
		return nil, errBadToken
	}
	return k, nil
}

type jwk struct {
	Kty, Crv, Kid, X, Y string
}

func (v *verifier) fetchLocked() error {
	v.fetched = time.Now()
	res, err := v.client.Get(v.jwksURL)
	if err != nil {
		return fmt.Errorf("auth: jwks: %w", err)
	}
	defer res.Body.Close()
	var set struct{ Keys []jwk }
	if res.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(res.Body, maxJWKSBytes)).Decode(&set) != nil {
		return fmt.Errorf("auth: jwks status %d", res.StatusCode)
	}
	keys := map[string]*ecdsa.PublicKey{}
	for _, j := range set.Keys {
		if pk := ecKey(j); pk != nil {
			keys[j.Kid] = pk
		}
	}
	v.keys = keys
	return nil
}

func ecKey(j jwk) *ecdsa.PublicKey {
	if j.Kty != "EC" || j.Crv != "P-256" {
		return nil
	}
	x, ex := base64.RawURLEncoding.DecodeString(j.X)
	y, ey := base64.RawURLEncoding.DecodeString(j.Y)
	if ex != nil || ey != nil || len(x) != 32 || len(y) != 32 {
		return nil
	}
	pk, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), append(append([]byte{4}, x...), y...))
	if err != nil {
		return nil // not a point on P-256
	}
	return pk
}
