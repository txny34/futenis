package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeIssuer plays Supabase Auth: an ES256 key published as a JWKS.
type fakeIssuer struct {
	key *ecdsa.PrivateKey
	kid string
	srv *httptest.Server
}

func newFakeIssuer(t *testing.T) *fakeIssuer {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeIssuer{key: k, kid: "test-kid"}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		pad := func(b *big.Int) string { return b64(b.FillBytes(make([]byte, 32))) }
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "EC", "crv": "P-256", "alg": "ES256", "kid": f.kid, "x": pad(k.X), "y": pad(k.Y),
		}}})
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func (f *fakeIssuer) token(t *testing.T, claims map[string]any) string {
	t.Helper()
	h, _ := json.Marshal(map[string]string{"alg": "ES256", "typ": "JWT", "kid": f.kid})
	c, _ := json.Marshal(claims)
	signed := b64(h) + "." + b64(c)
	sum := sha256.Sum256([]byte(signed))
	r, s, err := ecdsa.Sign(rand.Reader, f.key, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	sig := append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...)
	return signed + "." + b64(sig)
}

func goodClaims(iss string) map[string]any {
	return map[string]any{"sub": "8d1c7a3e-0000-4000-8000-000000000001", "aud": "authenticated", "iss": iss, "exp": time.Now().Add(time.Hour).Unix()}
}

func TestVerifyAcceptsSupabaseToken(t *testing.T) {
	f := newFakeIssuer(t)
	v := newVerifier(f.srv.URL, "https://proj.supabase.co/auth/v1")
	id, err := v.verify(f.token(t, goodClaims("https://proj.supabase.co/auth/v1")))
	if err != nil || id != "8d1c7a3e-0000-4000-8000-000000000001" {
		t.Fatalf("id=%q err=%v", id, err)
	}
}

func TestVerifyRejects(t *testing.T) {
	f := newFakeIssuer(t)
	iss := "https://proj.supabase.co/auth/v1"
	v := newVerifier(f.srv.URL, iss)
	expired := goodClaims(iss)
	expired["exp"] = time.Now().Add(-time.Minute).Unix()
	wrongAud := goodClaims(iss)
	wrongAud["aud"] = "anon"
	wrongIss := goodClaims("https://evil.example/auth/v1")
	good := f.token(t, goodClaims(iss))
	parts := strings.Split(good, ".")
	forgedClaims, _ := json.Marshal(map[string]any{"sub": "someone-else", "aud": "authenticated", "iss": iss, "exp": time.Now().Add(time.Hour).Unix()})
	for name, tok := range map[string]string{
		"expired":         f.token(t, expired),
		"wrong audience":  f.token(t, wrongAud),
		"wrong issuer":    f.token(t, wrongIss),
		"tampered claims": parts[0] + "." + b64(forgedClaims) + "." + parts[2],
		"alg none":        b64([]byte(`{"alg":"none","kid":"test-kid"}`)) + "." + parts[1] + ".",
		"garbage":         "not.a.jwt",
	} {
		if _, err := v.verify(tok); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	other := newFakeIssuer(t) // same kid, different key: signature must fail
	if _, err := v.verify(other.token(t, goodClaims(iss))); err == nil {
		t.Error("token signed by another key accepted")
	}
}
