package main

import (
	"context"
	"testing"
	"time"

	"github.com/coder/websocket"
	"net/http/httptest"
)

func dialQueue(ctx context.Context, t *testing.T, srv *httptest.Server, mode string) *websocket.Conn {
	t.Helper()
	c, _, err := websocket.Dial(ctx, "ws"+srv.URL[4:]+"/?queue="+mode, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.CloseNow() })
	return c
}

func TestQueuePairsArrivalsTwoByTwo(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newTestHub(make(chan time.Time))
	defer srv.Close()

	a := dialQueue(ctx, t, srv, "1v1")
	readMsg(ctx, t, a, msgWaiting, headerLen)
	b := dialQueue(ctx, t, srv, "1v1")
	readMsg(ctx, t, a, msgWelcome, welcomeLen)
	readMsg(ctx, t, b, msgWelcome, welcomeLen)
	// The third player starts a new match instead of hitting a full room.
	c := dialQueue(ctx, t, srv, "1v1")
	readMsg(ctx, t, c, msgWaiting, headerLen)
	d := dialQueue(ctx, t, srv, "1v1")
	readMsg(ctx, t, c, msgWelcome, welcomeLen)
	readMsg(ctx, t, d, msgWelcome, welcomeLen)
}

func TestQueueSkipsAbandonedWait(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newTestHub(make(chan time.Time))
	defer srv.Close()

	a := dialQueue(ctx, t, srv, "1v1")
	readMsg(ctx, t, a, msgWaiting, headerLen)
	a.Close(websocket.StatusNormalClosure, "")
	b := dialQueue(ctx, t, srv, "1v1")
	readMsg(ctx, t, b, msgWaiting, headerLen)
	c := dialQueue(ctx, t, srv, "1v1")
	readMsg(ctx, t, b, msgWelcome, welcomeLen)
	readMsg(ctx, t, c, msgWelcome, welcomeLen)
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
	if code := h.queueRoom(); roomCode.MatchString(code) {
		t.Fatalf("queue code %q passes the URL room filter", code)
	}
}
