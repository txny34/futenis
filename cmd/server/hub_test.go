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

func newTestHub(tick chan time.Time) *httptest.Server {
	return newTestHubCap(tick, maxRooms)
}

func newTestHubCap(tick chan time.Time, rooms int) *httptest.Server {
	h := newHub([]string{"*"}, log.New(io.Discard, "", 0))
	h.maxRooms = rooms
	h.ticker = func() (<-chan time.Time, func()) { return tick, func() {} }
	return httptest.NewServer(h)
}

func dial(ctx context.Context, t *testing.T, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	return dialRoom(ctx, t, srv, "ABC")
}

func dialRoom(ctx context.Context, t *testing.T, srv *httptest.Server, code string) *websocket.Conn {
	t.Helper()
	c, _, err := websocket.Dial(ctx, "ws"+srv.URL[4:]+"/?room="+code, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.CloseNow() })
	return c
}

func readMsg(ctx context.Context, t *testing.T, c *websocket.Conn, typ byte, n int) []byte {
	t.Helper()
	_, b, err := c.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != n || b[0] != typ || b[1] != protoVersion {
		t.Fatalf("got type %#x len %d, want %#x len %d", b[0], len(b), typ, n)
	}
	return b
}

func TestHubMatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tick := make(chan time.Time)
	srv := newTestHub(tick)
	defer srv.Close()

	c1 := dial(ctx, t, srv)
	readMsg(ctx, t, c1, msgWaiting, headerLen)
	c2 := dial(ctx, t, srv)
	if w := readMsg(ctx, t, c1, msgWelcome, welcomeLen); w[2] != 0 {
		t.Fatalf("host slot %d", w[2])
	}
	if w := readMsg(ctx, t, c2, msgWelcome, welcomeLen); w[2] != 1 {
		t.Fatalf("guest slot %d", w[2])
	}
	tick <- time.Time{}
	readMsg(ctx, t, c1, msgFrame, frameMsgLen)
	readMsg(ctx, t, c2, msgFrame, frameMsgLen)

	ping := make([]byte, pingMsgLen)
	putHeader(ping, msgPing)
	putF64(ping, 0, 42.25)
	if err := c1.Write(ctx, websocket.MessageBinary, ping); err != nil {
		t.Fatal(err)
	}
	if p := readMsg(ctx, t, c1, msgPong, pingMsgLen); f64(p, 0) != 42.25 {
		t.Fatalf("pong payload %v", f64(p, 0))
	}

	in := inputMsg([inputFloats]float64{1, 0, 3, 0, 0, 0})
	if err := c2.Write(ctx, websocket.MessageBinary, in); err != nil {
		t.Fatal(err)
	}
	c2.CloseNow()
	readMsg(ctx, t, c1, msgPeerLeft, headerLen)
}

func TestHubThirdPeerRejected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newTestHub(make(chan time.Time))
	defer srv.Close()
	c1 := dial(ctx, t, srv)
	readMsg(ctx, t, c1, msgWaiting, headerLen)
	c2 := dial(ctx, t, srv)
	// WELCOME proves c2 joined: only then is the room full for c3 (dial returns
	// at the handshake, before the server-side join runs).
	readMsg(ctx, t, c2, msgWelcome, welcomeLen)
	c3 := dial(ctx, t, srv)
	if _, _, err := c3.Read(ctx); websocket.CloseStatus(err) != closeFull {
		t.Fatalf("third peer err %v", err)
	}
}

func TestHubBadRoom(t *testing.T) {
	srv := newTestHub(make(chan time.Time))
	defer srv.Close()
	res, err := http.Get(srv.URL + "/?room=bad!")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestHubServerFull(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newTestHubCap(make(chan time.Time), 1)
	defer srv.Close()
	c1 := dialRoom(ctx, t, srv, "AAA")
	readMsg(ctx, t, c1, msgWaiting, headerLen)
	c2 := dialRoom(ctx, t, srv, "BBB")
	if _, _, err := c2.Read(ctx); websocket.CloseStatus(err) != closeBusy {
		t.Fatalf("second room err %v", err)
	}
}

func TestHubBadMessage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newTestHub(make(chan time.Time))
	defer srv.Close()
	for _, tc := range []struct {
		typ websocket.MessageType
		b   []byte
	}{
		{websocket.MessageBinary, []byte{1, 2, 3}},
		{websocket.MessageText, inputMsg([inputFloats]float64{})},
	} {
		c := dial(ctx, t, srv)
		readMsg(ctx, t, c, msgWaiting, headerLen)
		if err := c.Write(ctx, tc.typ, tc.b); err != nil {
			t.Fatal(err)
		}
		if _, _, err := c.Read(ctx); websocket.CloseStatus(err) != websocket.StatusUnsupportedData {
			t.Fatalf("type %v err %v", tc.typ, err)
		}
	}
}
