package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func newTestHubWith(set func(h *hub)) *httptest.Server {
	h := newHub([]string{"*"}, log.New(io.Discard, "", 0))
	h.ticker = func() (<-chan time.Time, func()) { return make(chan time.Time), func() {} }
	set(h)
	return httptest.NewServer(h)
}

func TestConnLimiter(t *testing.T) {
	l := newConnLimiter(3, 2)
	for i := range 2 {
		if !l.acquire("a") {
			t.Fatalf("acquire %d from a rejected", i)
		}
	}
	if l.acquire("a") {
		t.Fatal("third from a must hit the per-IP cap")
	}
	if !l.acquire("b") {
		t.Fatal("b must pass")
	}
	if l.acquire("c") {
		t.Fatal("c must hit the global cap")
	}
	l.release("a")
	l.release("a")
	if _, ok := l.perIP["a"]; ok {
		t.Fatal("released IP must be forgotten")
	}
	if !l.acquire("c") {
		t.Fatal("c must pass after releases")
	}
}

func TestRateWindow(t *testing.T) {
	var w rateWindow
	t0 := time.Unix(100, 0)
	for i := 0; i < maxMsgsPerWindow; i++ {
		if !w.allow(t0) {
			t.Fatalf("message %d rejected", i)
		}
	}
	if w.allow(t0) {
		t.Fatal("message over the cap allowed")
	}
	if !w.allow(t0.Add(msgWindow)) {
		t.Fatal("a new window must reset the count")
	}
}

func TestClientIP(t *testing.T) {
	for addr, want := range map[string]string{"1.2.3.4:5": "1.2.3.4", "[::1]:80": "::1", "weird": "weird"} {
		if got := clientIP(&http.Request{RemoteAddr: addr}, false); got != want {
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
		if got := clientIP(req, true); got != want {
			t.Errorf("%q: got %q want %q", xff, got, want)
		}
		if got := clientIP(req, false); got != "10.0.0.1" {
			t.Errorf("untrusted %q: got %q", xff, got)
		}
	}
}

func expectClose(ctx context.Context, t *testing.T, c *websocket.Conn, code websocket.StatusCode) {
	t.Helper()
	for {
		_, _, err := c.Read(ctx)
		if err == nil {
			continue // WAITING etc.
		}
		if got := websocket.CloseStatus(err); got != code {
			t.Fatalf("close %v (err %v), want %v", got, err, code)
		}
		return
	}
}

func TestHubIdleKick(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newTestHubWith(func(h *hub) { h.idle = 50 * time.Millisecond })
	defer srv.Close()
	c1 := dial(ctx, t, srv)
	readMsg(ctx, t, c1, msgWaiting, headerLen)
	c2 := dial(ctx, t, srv)
	go keepAlive(ctx, c2) // only the host goes idle
	expectClose(ctx, t, c1, closeIdle)
}

func keepAlive(ctx context.Context, c *websocket.Conn) {
	in := inputMsg([inputFloats]float64{})
	for ctx.Err() == nil {
		if c.Write(ctx, websocket.MessageBinary, in) != nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestHubWaitingHostNotIdle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newTestHubWith(func(h *hub) { h.idle = 20 * time.Millisecond; h.wait = 300 * time.Millisecond })
	defer srv.Close()
	start := time.Now()
	expectClose(ctx, t, dial(ctx, t, srv), closeNoRival)
	if time.Since(start) < 250*time.Millisecond {
		t.Fatal("waiting host closed before the no-rival timeout")
	}
}

// A host that waited silently (hidden tab) longer than the idle timeout gets a
// full idle window once the rival arrives, not an instant kick.
func TestHubIdleClockStartsWithMatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newTestHubWith(func(h *hub) { h.idle = 200 * time.Millisecond; h.wait = 5 * time.Second })
	defer srv.Close()
	c1 := dial(ctx, t, srv)
	readMsg(ctx, t, c1, msgWaiting, headerLen)
	time.Sleep(400 * time.Millisecond) // silent wait: twice the idle timeout
	c2 := dial(ctx, t, srv)
	go keepAlive(ctx, c2)
	joined := time.Now()
	expectClose(ctx, t, c1, closeIdle)
	if time.Since(joined) < 150*time.Millisecond {
		t.Fatalf("host kicked %v after the match started: waiting time counted as idle", time.Since(joined))
	}
}

func TestHubNoRival(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newTestHubWith(func(h *hub) { h.wait = 50 * time.Millisecond })
	defer srv.Close()
	expectClose(ctx, t, dial(ctx, t, srv), closeNoRival)
}

func TestHubRateLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newTestHubWith(func(*hub) {})
	defer srv.Close()
	c := dial(ctx, t, srv)
	readMsg(ctx, t, c, msgWaiting, headerLen)
	in := inputMsg([inputFloats]float64{})
	for i := 0; i <= maxMsgsPerWindow; i++ {
		if err := c.Write(ctx, websocket.MessageBinary, in); err != nil {
			break // the server may close before the last write lands
		}
	}
	expectClose(ctx, t, c, closeRate)
}

func TestHubPerIPLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newTestHubWith(func(h *hub) { h.conns = newConnLimiter(maxConns, 1) })
	defer srv.Close()
	c := dialRoom(ctx, t, srv, "AAA")
	readMsg(ctx, t, c, msgWaiting, headerLen)
	_, res, err := websocket.Dial(ctx, "ws"+srv.URL[4:]+"/?room=BBB", nil)
	if err == nil || res == nil || res.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second conn from same IP: err %v res %v", err, res)
	}
}

func TestClientIPFlyHeaderWins(t *testing.T) {
	req := &http.Request{RemoteAddr: "10.0.0.1:443", Header: http.Header{}}
	req.Header.Set("X-Forwarded-For", "6.6.6.6, 7.7.7.7")
	req.Header.Set("Fly-Client-IP", "9.9.9.9")
	if got := clientIP(req, true); got != "9.9.9.9" {
		t.Fatalf("got %q", got)
	}
	if got := clientIP(req, false); got != "10.0.0.1" {
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
		if got := limitKey(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}
