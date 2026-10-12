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
	"github.com/txny34/futenis/cmd/server/internal/peertest"
	"github.com/txny34/futenis/cmd/server/internal/ranked"
	"github.com/txny34/futenis/cmd/server/internal/wire"
	"github.com/txny34/futenis/sim"
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
	readMsg(ctx, t, a, wire.MsgWaiting, wire.HeaderLen)
	b := dialQueue(ctx, t, srv, "10.0.0.2")
	readWelcome(ctx, t, a)
	readWelcome(ctx, t, b)
	// The third player starts a new match instead of hitting a full room.
	c := dialQueue(ctx, t, srv, "10.0.0.3")
	readMsg(ctx, t, c, wire.MsgWaiting, wire.HeaderLen)
	d := dialQueue(ctx, t, srv, "10.0.0.4")
	readWelcome(ctx, t, c)
	readWelcome(ctx, t, d)
}

func TestQueueSkipsAbandonedWait(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var h *hub
	srv := newTestHubWith(func(x *hub) { x.proxied, h = true, x })
	defer srv.Close()

	a := dialQueue(ctx, t, srv, "10.0.0.5")
	readMsg(ctx, t, a, wire.MsgWaiting, wire.HeaderLen)
	a.Close(websocket.StatusNormalClosure, "")
	waitNoRooms(ctx, t, h) // Close returns before the server ran a's leave: b could meet the leaving a
	b := dialQueue(ctx, t, srv, "10.0.0.6")
	readMsg(ctx, t, b, wire.MsgWaiting, wire.HeaderLen)
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
	if code, _ := h.queueRoom("1v1", "k", ranked.EloDefault); roomCode.MatchString(code) {
		t.Fatalf("queue code %q passes the URL room filter", code)
	}
}

func TestQueueOnePerIP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newQueueHub()
	defer srv.Close()
	a := dialQueue(ctx, t, srv, "10.0.0.8")
	readMsg(ctx, t, a, wire.MsgWaiting, wire.HeaderLen)
	b := dialQueue(ctx, t, srv, "10.0.0.8") // same IP: never paired with itself
	expectClose(ctx, t, b, wire.CloseFull)
}

func TestQueueRejoinAfterLeaving(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var h *hub
	srv := newTestHubWith(func(x *hub) { x.proxied, h = true, x })
	defer srv.Close()
	a := dialQueue(ctx, t, srv, "10.0.1.1")
	readMsg(ctx, t, a, wire.MsgWaiting, wire.HeaderLen)
	a.Close(websocket.StatusNormalClosure, "")
	waitNoRooms(ctx, t, h) // the server saw the leave
	again := dialQueue(ctx, t, srv, "10.0.1.1")
	readMsg(ctx, t, again, wire.MsgWaiting, wire.HeaderLen)
}

// A rejected duplicate must not erase the original waiter from the queue: a
// third connection from the same client is still rejected, never self-paired.
func TestQueueDuplicateRejectionKeepsOwner(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newQueueHub()
	defer srv.Close()
	a := dialQueue(ctx, t, srv, "10.0.3.1")
	readMsg(ctx, t, a, wire.MsgWaiting, wire.HeaderLen)
	b := dialQueue(ctx, t, srv, "10.0.3.1")
	expectClose(ctx, t, b, wire.CloseFull)
	time.Sleep(50 * time.Millisecond) // the server runs b's cleanup
	c := dialQueue(ctx, t, srv, "10.0.3.1")
	expectClose(ctx, t, c, wire.CloseFull)
}

// The rival can fill and start the room between a player's join and its queue
// claim: that player is in a live match and must stay (leaving ended the
// match right after WELCOME). A waiter of a room no longer open must not stay.
func TestQueueSettleWait(t *testing.T) {
	cases := []struct {
		name         string
		open, rival  bool // queue entry still open / a rival filled the room
		closed, want bool
	}{
		{"claimable", true, false, false, true},
		{"rival started it", false, true, false, true},
		{"entry gone, still waiting", false, false, false, false},
		{"started then ended", false, true, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHub([]string{"*"}, log.New(io.Discard, "", 0))
			code, _ := h.queueRoom("1v1", "a", ranked.EloDefault)
			a := &peertest.Peer{}
			r, slot, _, _ := h.join(code, sim.Mode1v1, a)
			if tc.rival {
				h.join(code, sim.Mode1v1, &peertest.Peer{})
			}
			if !tc.open {
				h.closeQueueRoom("1v1", code)
			}
			if tc.closed {
				r.Leave(slot, a, false)
			}
			if got := h.settleWait("1v1", code, "a", r); got != tc.want {
				t.Fatalf("settleWait = %v, want %v", got, tc.want)
			}
		})
	}
}

// waitNoRooms polls until the hub forgot every room (the leaves ran).
func waitNoRooms(ctx context.Context, t *testing.T, h *hub) {
	t.Helper()
	for {
		h.mu.Lock()
		n := len(h.rooms)
		h.mu.Unlock()
		if n == 0 {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("%d rooms still open", n)
		case <-time.After(time.Millisecond):
		}
	}
}
