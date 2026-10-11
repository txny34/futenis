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
	"github.com/txny34/futenis/cmd/server/internal/wire"
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
	if len(b) != n || b[0] != typ || b[1] != wire.Version {
		t.Fatalf("got type %#x len %d, want %#x len %d", b[0], len(b), typ, n)
	}
	return b
}

// readWelcome reads WELCOME and the SEAT (rejoin token) that follows it.
func readWelcome(ctx context.Context, t *testing.T, c *websocket.Conn) []byte {
	t.Helper()
	w := readMsg(ctx, t, c, wire.MsgWelcome, wire.WelcomeLen)
	readMsg(ctx, t, c, wire.MsgSeat, wire.HeaderLen+tokenLen)
	return w
}

func TestHubMatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tick := make(chan time.Time)
	srv := newTestHub(tick)
	defer srv.Close()

	c1 := dial(ctx, t, srv)
	readMsg(ctx, t, c1, wire.MsgWaiting, wire.HeaderLen)
	c2 := dial(ctx, t, srv)
	if w := readWelcome(ctx, t, c1); w[2] != 0 {
		t.Fatalf("host slot %d", w[2])
	}
	if w := readWelcome(ctx, t, c2); w[2] != 1 {
		t.Fatalf("guest slot %d", w[2])
	}
	tick <- time.Time{}
	readMsg(ctx, t, c1, wire.MsgFrame, wire.FrameLen)
	readMsg(ctx, t, c2, wire.MsgFrame, wire.FrameLen)

	ping := make([]byte, wire.PingLen)
	wire.PutHeader(ping, wire.MsgPing)
	wire.PutF64(ping, 0, 42.25)
	if err := c1.Write(ctx, websocket.MessageBinary, ping); err != nil {
		t.Fatal(err)
	}
	if p := readMsg(ctx, t, c1, wire.MsgPong, wire.PingLen); wire.F64(p, 0) != 42.25 {
		t.Fatalf("pong payload %v", wire.F64(p, 0))
	}

	in := wire.EncodeInput([wire.InputFloats]float64{1, 0, 3, 0, 0, 0})
	if err := c2.Write(ctx, websocket.MessageBinary, in); err != nil {
		t.Fatal(err)
	}
	_ = c2.Close(websocket.StatusNormalClosure, "") // leaving on purpose: no seat held
	readMsg(ctx, t, c1, wire.MsgPeerLeft, wire.HeaderLen)
}

func TestHubThirdPeerRejected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newTestHub(make(chan time.Time))
	defer srv.Close()
	c1 := dial(ctx, t, srv)
	readMsg(ctx, t, c1, wire.MsgWaiting, wire.HeaderLen)
	c2 := dial(ctx, t, srv)
	// WELCOME proves c2 joined: only then is the room full for c3 (dial returns
	// at the handshake, before the server-side join runs).
	readWelcome(ctx, t, c2)
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
	readMsg(ctx, t, c1, wire.MsgWaiting, wire.HeaderLen)
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
		room string
		typ  websocket.MessageType
		b    []byte
	}{
		// One room per case: the previous peer's server-side leave may not have
		// run yet when the next one dials, so a shared room could WELCOME it.
		{"BADA", websocket.MessageBinary, []byte{1, 2, 3}},
		{"BADB", websocket.MessageText, wire.EncodeInput([wire.InputFloats]float64{})},
	} {
		c := dialRoom(ctx, t, srv, tc.room)
		readMsg(ctx, t, c, wire.MsgWaiting, wire.HeaderLen)
		if err := c.Write(ctx, tc.typ, tc.b); err != nil {
			t.Fatal(err)
		}
		if _, _, err := c.Read(ctx); websocket.CloseStatus(err) != websocket.StatusUnsupportedData {
			t.Fatalf("type %v err %v", tc.typ, err)
		}
	}
}
