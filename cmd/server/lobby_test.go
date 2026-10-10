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

func newLobbyServer(t *testing.T, tick chan time.Time) (*httptest.Server, *hub) {
	t.Helper()
	h := newHub([]string{"*"}, log.New(io.Discard, "", 0))
	h.proxied = true
	h.lobbyTicker = func() (<-chan time.Time, func()) { return tick, func() {} }
	srv := httptest.NewServer(http.HandlerFunc(h.ServeLobby))
	t.Cleanup(func() { close(h.done); srv.Close() })
	return srv, h
}

func dialLobby(ctx context.Context, t *testing.T, srv *httptest.Server, ip string) *websocket.Conn {
	t.Helper()
	c, _, err := websocket.Dial(ctx, "ws"+srv.URL[4:], &websocket.DialOptions{HTTPHeader: http.Header{"Fly-Client-IP": {ip}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.CloseNow() })
	return c
}

func write(ctx context.Context, t *testing.T, c *websocket.Conn, b []byte) {
	t.Helper()
	if err := c.Write(ctx, websocket.MessageBinary, b); err != nil {
		t.Fatal(err)
	}
}

func readType(ctx context.Context, t *testing.T, c *websocket.Conn, typ byte) []byte {
	t.Helper()
	for {
		_, b, err := c.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if b[0] == typ {
			return b
		}
	}
}

func nameMsg(s string) []byte {
	b := make([]byte, headerLen+len(s))
	putHeader(b, msgName)
	copy(b[headerLen:], s)
	return b
}

func TestLobbyRelaysOthersPositionsAndNames(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tick := make(chan time.Time)
	srv, _ := newLobbyServer(t, tick)
	a := dialLobby(ctx, t, srv, "10.5.0.1")
	b := dialLobby(ctx, t, srv, "10.5.0.2")
	write(ctx, t, a, nameMsg(" txny\x0044 "))
	if got := readType(ctx, t, b, msgLobbyName); string(got[headerLen+8:]) != "txny44" {
		t.Fatalf("name %q", got[headerLen+8:])
	}
	write(ctx, t, a, presenceMsg(3, 0, 999, 0))
	time.Sleep(30 * time.Millisecond) // the server stores a's position
	tick <- time.Time{}
	r := readType(ctx, t, b, msgRoster)
	if f64(r, 0) != 1 || f64(r, 2) != 3 || f64(r, 4) != lobbyHalfZ {
		t.Fatalf("roster count=%v x=%v z=%v", f64(r, 0), f64(r, 2), f64(r, 4))
	}
	if self := readType(ctx, t, a, msgRoster); f64(self, 0) != 0 {
		t.Fatalf("a sees itself or b without a position: count %v", f64(self, 0))
	}
	a.Close(websocket.StatusNormalClosure, "")
	if g := readType(ctx, t, b, msgGone); f64(g, 0) != f64(r, 1) {
		t.Fatalf("gone id %v want %v", f64(g, 0), f64(r, 1))
	}
}

func TestLobbyRejectsGarbage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv, _ := newLobbyServer(t, make(chan time.Time))
	a := dialLobby(ctx, t, srv, "10.5.1.1")
	write(ctx, t, a, []byte{0x10, 1, 0, 0})
	if _, _, err := a.Read(ctx); websocket.CloseStatus(err) != websocket.StatusUnsupportedData {
		t.Fatalf("garbage not rejected: %v", err)
	}
}

func TestLobbyFull(t *testing.T) {
	l := newLobby()
	for i := 0; i < maxLobby; i++ {
		if _, ok := l.join(&fakePeer{}); !ok {
			t.Fatalf("join %d rejected", i)
		}
	}
	if _, ok := l.join(&fakePeer{}); ok {
		t.Fatal("lobby over capacity")
	}
}

func TestLobbySilentPeerIsDropped(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv, h := newLobbyServer(t, make(chan time.Time))
	h.lobbyIdle = 50 * time.Millisecond
	a := dialLobby(ctx, t, srv, "10.5.2.1")
	if _, _, err := a.Read(ctx); err == nil {
		t.Fatal("silent lobby peer kept")
	}
}

// Name spam: only one name change per window reaches the others.
func TestLobbyNameChangesAreThrottled(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv, _ := newLobbyServer(t, make(chan time.Time))
	a := dialLobby(ctx, t, srv, "10.6.0.1")
	b := dialLobby(ctx, t, srv, "10.6.0.2")
	for i := 0; i < 20; i++ {
		write(ctx, t, a, nameMsg(string(rune('a'+i%2))))
	}
	readType(ctx, t, b, msgLobbyName)
	short, cancelShort := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancelShort()
	if _, _, err := b.Read(short); err == nil {
		t.Fatal("a second name change got through inside the window")
	}
}

// Floods beyond the lobby's own (much lower) message cap are closed.
func TestLobbyRateLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv, _ := newLobbyServer(t, make(chan time.Time))
	a := dialLobby(ctx, t, srv, "10.6.1.1")
	for i := 0; i < maxLobbyMsgs+5; i++ {
		_ = a.Write(ctx, websocket.MessageBinary, presenceMsg(0, 0, 0, 0))
	}
	if _, _, err := a.Read(ctx); websocket.CloseStatus(err) != closeRate {
		t.Fatalf("flood not closed: %v", err)
	}
}

// A full lobby opens another one instead of turning people away.
func TestLobbyShards(t *testing.T) {
	h := newHub([]string{"*"}, log.New(io.Discard, "", 0))
	defer close(h.done)
	seen := map[*lobby]int{}
	for i := 0; i < maxLobby+1; i++ {
		l, _, ok := h.joinLobby(&fakePeer{})
		if !ok {
			t.Fatalf("join %d rejected", i)
		}
		seen[l]++
	}
	if len(seen) != 2 {
		t.Fatalf("%d lobbies for %d people", len(seen), maxLobby+1)
	}
}
