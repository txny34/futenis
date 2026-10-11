// Package authtest plays Supabase Auth in tests: an ES256 key published as a
// JWKS and tokens signed with it.
package authtest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Issuer plays Supabase Auth: an ES256 key published as a JWKS.
type Issuer struct {
	key *ecdsa.PrivateKey
	kid string
	Srv *httptest.Server
}

func NewIssuer(t *testing.T) *Issuer {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	f := &Issuer{key: k, kid: "test-kid"}
	f.Srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		raw, _ := k.PublicKey.Bytes() // 0x04 || X || Y
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "EC", "crv": "P-256", "alg": "ES256", "kid": f.kid, "x": B64(raw[1:33]), "y": B64(raw[33:]),
		}}})
	}))
	t.Cleanup(f.Srv.Close)
	return f
}

// B64 is unpadded base64url, the JWT encoding.
func B64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func (f *Issuer) Token(t *testing.T, claims map[string]any) string {
	t.Helper()
	h, _ := json.Marshal(map[string]string{"alg": "ES256", "typ": "JWT", "kid": f.kid})
	c, _ := json.Marshal(claims)
	signed := B64(h) + "." + B64(c)
	sum := sha256.Sum256([]byte(signed))
	r, s, err := ecdsa.Sign(rand.Reader, f.key, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	sig := append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...)
	return signed + "." + B64(sig)
}

func GoodClaims(iss string) map[string]any {
	return map[string]any{"sub": "8d1c7a3e-0000-4000-8000-000000000001", "aud": "authenticated", "iss": iss, "exp": time.Now().Add(time.Hour).Unix()}
}
