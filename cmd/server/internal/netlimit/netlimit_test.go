package netlimit

import (
	"net/http"
	"testing"
	"time"
)

const testMax = 242

func TestConnLimiter(t *testing.T) {
	l := NewConnLimiter(3, 2)
	for i := range 2 {
		if !l.Acquire("a") {
			t.Fatalf("acquire %d from a rejected", i)
		}
	}
	if l.Acquire("a") {
		t.Fatal("third from a must hit the per-IP cap")
	}
	if !l.Acquire("b") {
		t.Fatal("b must pass")
	}
	if l.Acquire("c") {
		t.Fatal("c must hit the global cap")
	}
	l.Release("a")
	l.Release("a")
	if _, ok := l.perIP["a"]; ok {
		t.Fatal("released IP must be forgotten")
	}
	if !l.Acquire("c") {
		t.Fatal("c must pass after releases")
	}
}

func TestRateWindow(t *testing.T) {
	var w RateWindow
	t0 := time.Unix(100, 0)
	for i := 0; i < testMax; i++ {
		if !w.AllowN(t0, testMax) {
			t.Fatalf("message %d rejected", i)
		}
	}
	if w.AllowN(t0, testMax) {
		t.Fatal("message over the cap allowed")
	}
	if !w.AllowN(t0.Add(Window), testMax) {
		t.Fatal("a new window must reset the count")
	}
}

func TestClientIP(t *testing.T) {
	for addr, want := range map[string]string{"1.2.3.4:5": "1.2.3.4", "[::1]:80": "::1", "weird": "weird"} {
		if got := ClientIP(&http.Request{RemoteAddr: addr}, false); got != want {
			t.Errorf("%q: got %q want %q", addr, got, want)
		}
	}
}

func TestClientIPTrustProxy(t *testing.T) {
	for xff, want := range map[string]string{
		"":                   "10.0.0.1",
		"9.9.9.9":            "9.9.9.9",
		"6.6.6.6, 9.9.9.9":   "9.9.9.9", // client-spoofed entries come first; the proxy appends the real one
		"6.6.6.6,  9.9.9.9 ": "9.9.9.9",
		"6.6.6.6, ":          "10.0.0.1",
	} {
		req := &http.Request{RemoteAddr: "10.0.0.1:443", Header: http.Header{}}
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}
		if got := ClientIP(req, true); got != want {
			t.Errorf("%q: got %q want %q", xff, got, want)
		}
		if got := ClientIP(req, false); got != "10.0.0.1" {
			t.Errorf("untrusted %q: got %q", xff, got)
		}
	}
}

func TestClientIPFlyHeaderWins(t *testing.T) {
	req := &http.Request{RemoteAddr: "10.0.0.1:443", Header: http.Header{}}
	req.Header.Set("X-Forwarded-For", "6.6.6.6, 7.7.7.7")
	req.Header.Set("Fly-Client-IP", "9.9.9.9")
	if got := ClientIP(req, true); got != "9.9.9.9" {
		t.Fatalf("got %q", got)
	}
	if got := ClientIP(req, false); got != "10.0.0.1" {
		t.Fatalf("untrusted: got %q", got)
	}
}

func TestLimitKeyBucketsIPv6(t *testing.T) {
	for in, want := range map[string]string{
		"9.9.9.9":                 "9.9.9.9",
		"2001:db8:1:2:aaaa::1":    "2001:db8:1:2::/64",
		"2001:db8:1:2:bbbb:cc::9": "2001:db8:1:2::/64",
		"::ffff:9.9.9.9":          "9.9.9.9",
		"garbage":                 "garbage",
	} {
		if got := LimitKey(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}
