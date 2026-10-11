package main

import (
	"context"
	"testing"
	"time"

	"github.com/coder/websocket"
	"net/http"
	"net/http/httptest"
)

// newQueueHub trusts Fly-Client-IP so each test client can have its own IP.
func newQueueHub() *httptest.Server {
	return newTestHubWith(func(h *hub) { h.proxied = true })
}

func dialQueue(ctx context.Context, t *testing.T, srv *httptest.Server, ip string) *websocket.Conn {
	t.Helper()
	hdr := http.Header{"Fly-Client-IP": {ip}}
	c, _, err := websocket.Dial(ctx, "ws"+srv.URL[4:]+"/?queue=1v1", &websocket.DialOptions{HTTPHeader: hdr})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.CloseNow() })
	return c
}

func TestQueuePairsArrivalsTwoByTwo(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newQueueHub()
	defer srv.Close()

	a := dialQueue(ctx, t, srv, "10.0.0.1")
	readMsg(ctx, t, a, msgWaiting, headerLen)
	b := dialQueue(ctx, t, srv, "10.0.0.2")
	readWelcome(ctx, t, a)
	readWelcome(ctx, t, b)
	// The third player starts a new match instead of hitting a full room.
	c := dialQueue(ctx, t, srv, "10.0.0.3")
	readMsg(ctx, t, c, msgWaiting, headerLen)
	d := dialQueue(ctx, t, srv, "10.0.0.4")
	readWelcome(ctx, t, c)
	readWelcome(ctx, t, d)
}

func TestQueueSkipsAbandonedWait(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newQueueHub()
	defer srv.Close()

	a := dialQueue(ctx, t, srv, "10.0.0.5")
	readMsg(ctx, t, a, msgWaiting, headerLen)
	a.Close(websocket.StatusNormalClosure, "")
	b := dialQueue(ctx, t, srv, "10.0.0.6")
	readMsg(ctx, t, b, msgWaiting, headerLen)
	c := dialQueue(ctx, t, srv, "10.0.0.7")
	readWelcome(ctx, t, b)
	readWelcome(ctx, t, c)
}

func TestQueueRejectsUnknownMode(t *testing.T) {
	srv := newTestHub(make(chan time.Time))
	defer srv.Close()
	res, err := srv.Client().Get(srv.URL + "/?queue=3v3")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestQueueCodesAreNotJoinableByURL(t *testing.T) {
	h := newHub([]string{"*"}, nil)
	if code, _ := h.queueRoom("1v1", "k"); roomCode.MatchString(code) {
		t.Fatalf("queue code %q passes the URL room filter", code)
	}
}

func TestQueueOnePerIP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newQueueHub()
	defer srv.Close()
	a := dialQueue(ctx, t, srv, "10.0.0.8")
	readMsg(ctx, t, a, msgWaiting, headerLen)
	b := dialQueue(ctx, t, srv, "10.0.0.8") // same IP: never paired with itself
	expectClose(ctx, t, b, closeFull)
}

func TestQueueRejoinAfterLeaving(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newQueueHub()
	defer srv.Close()
	a := dialQueue(ctx, t, srv, "10.0.1.1")
	readMsg(ctx, t, a, msgWaiting, headerLen)
	a.Close(websocket.StatusNormalClosure, "")
	time.Sleep(50 * time.Millisecond) // let the server see the leave
	again := dialQueue(ctx, t, srv, "10.0.1.1")
	readMsg(ctx, t, again, msgWaiting, headerLen)
}

// A rejected duplicate must not erase the original waiter from the queue: a
// third connection from the same client is still rejected, never self-paired.
func TestQueueDuplicateRejectionKeepsOwner(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newQueueHub()
	defer srv.Close()
	a := dialQueue(ctx, t, srv, "10.0.3.1")
	readMsg(ctx, t, a, msgWaiting, headerLen)
	b := dialQueue(ctx, t, srv, "10.0.3.1")
	expectClose(ctx, t, b, closeFull)
	time.Sleep(50 * time.Millisecond) // the server runs b's cleanup
	c := dialQueue(ctx, t, srv, "10.0.3.1")
	expectClose(ctx, t, c, closeFull)
}
