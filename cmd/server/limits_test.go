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
	"github.com/txny34/futenis/cmd/server/internal/netlimit"
)

func newTestHubWith(set func(h *hub)) *httptest.Server {
	h := newHub([]string{"*"}, log.New(io.Discard, "", 0))
	h.ticker = func() (<-chan time.Time, func()) { return make(chan time.Time), func() {} }
	set(h)
	return httptest.NewServer(h)
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
	srv := newTestHubWith(func(h *hub) { h.conns = netlimit.NewConnLimiter(maxConns, 1) })
	defer srv.Close()
	c := dialRoom(ctx, t, srv, "AAA")
	readMsg(ctx, t, c, msgWaiting, headerLen)
	_, res, err := websocket.Dial(ctx, "ws"+srv.URL[4:]+"/?room=BBB", nil)
	if err == nil || res == nil || res.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second conn from same IP: err %v res %v", err, res)
	}
}
