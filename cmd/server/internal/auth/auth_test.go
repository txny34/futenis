package auth

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/txny34/futenis/cmd/server/internal/auth/authtest"
	"github.com/txny34/futenis/cmd/server/internal/names"
)

func TestVerifyAcceptsSupabaseToken(t *testing.T) {
	f := authtest.NewIssuer(t)
	v := NewVerifier(f.Srv.URL, "https://proj.supabase.co/auth/v1")
	c := authtest.GoodClaims("https://proj.supabase.co/auth/v1")
	c["user_metadata"] = map[string]any{"full_name": "Ana Pérez", "custom_claims": map[string]any{"global_name": "anita"}}
	acc, err := v.Verify(f.Token(t, c))
	if err != nil || acc.ID != "8d1c7a3e-0000-4000-8000-000000000001" || acc.Name != "anita" {
		t.Fatalf("account=%+v err=%v", acc, err)
	}
	c["user_metadata"] = map[string]any{"full_name": "Pendejo Total"}
	if acc, _ := v.Verify(f.Token(t, c)); acc.Name != names.Fallback {
		t.Fatalf("offensive name kept: %q", acc.Name)
	}
}

func TestVerifyRejects(t *testing.T) {
	f := authtest.NewIssuer(t)
	iss := "https://proj.supabase.co/auth/v1"
	v := NewVerifier(f.Srv.URL, iss)
	expired := authtest.GoodClaims(iss)
	expired["exp"] = time.Now().Add(-time.Minute).Unix()
	wrongAud := authtest.GoodClaims(iss)
	wrongAud["aud"] = "anon"
	wrongIss := authtest.GoodClaims("https://evil.example/auth/v1")
	good := f.Token(t, authtest.GoodClaims(iss))
	parts := strings.Split(good, ".")
	forgedClaims, _ := json.Marshal(map[string]any{"sub": "someone-else", "aud": "authenticated", "iss": iss, "exp": time.Now().Add(time.Hour).Unix()})
	for name, tok := range map[string]string{
		"expired":         f.Token(t, expired),
		"wrong audience":  f.Token(t, wrongAud),
		"wrong issuer":    f.Token(t, wrongIss),
		"tampered claims": parts[0] + "." + authtest.B64(forgedClaims) + "." + parts[2],
		"alg none":        authtest.B64([]byte(`{"alg":"none","kid":"test-kid"}`)) + "." + parts[1] + ".",
		"garbage":         "not.a.jwt",
	} {
		if _, err := v.Verify(tok); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	other := authtest.NewIssuer(t) // same kid, different key: signature must fail
	if _, err := v.Verify(other.Token(t, authtest.GoodClaims(iss))); err == nil {
		t.Error("token signed by another key accepted")
	}
}
