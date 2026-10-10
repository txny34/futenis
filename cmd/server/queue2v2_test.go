package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func newFillHub(tick chan time.Time, fill time.Duration) *httptest.Server {
	return newTestHubWith(func(h *hub) {
		h.proxied = true
		h.fill = fill
		h.ticker = func() (<-chan time.Time, func()) { return tick, func() {} }
	})
}

func dial2v2(ctx context.Context, t *testing.T, srv *httptest.Server, ip string) *websocket.Conn {
	t.Helper()
	hdr := http.Header{"Fly-Client-IP": {ip}}
	c, _, err := websocket.Dial(ctx, "ws"+srv.URL[4:]+"/?queue=2v2", &websocket.DialOptions{HTTPHeader: hdr})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.CloseNow() })
	return c
}

func TestQueue2v2FourPlayersGetSeats0To3(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tick := make(chan time.Time)
	srv := newFillHub(tick, time.Hour)
	defer srv.Close()
	var cs []*websocket.Conn
	for i, ip := range []string{"10.2.0.1", "10.2.0.2", "10.2.0.3", "10.2.0.4"} {
		c := dial2v2(ctx, t, srv, ip)
		if i < 3 {
			readMsg(ctx, t, c, msgWaiting, headerLen)
		}
		cs = append(cs, c)
	}
	for i, c := range cs {
		if w := readMsg(ctx, t, c, msgWelcome, welcomeLen); int(w[2]) != seats2v2[i] {
			t.Fatalf("player %d got seat %d, want %d (first two are rivals)", i, w[2], seats2v2[i])
		}
	}
	tick <- time.Time{}
	for _, c := range cs {
		readMsg(ctx, t, c, msgFrame, frameMsgLen)
	}
}

func TestQueue2v2BotsFillAfterWait(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newFillHub(make(chan time.Time), 50*time.Millisecond)
	defer srv.Close()
	a := dial2v2(ctx, t, srv, "10.2.1.1")
	readMsg(ctx, t, a, msgWaiting, headerLen)
	b := dial2v2(ctx, t, srv, "10.2.1.2")
	readMsg(ctx, t, b, msgWaiting, headerLen)
	readMsg(ctx, t, a, msgWelcome, welcomeLen) // two people + two bots
	readMsg(ctx, t, b, msgWelcome, welcomeLen)
}

func TestQueue2v2LoneWaiterNeverStartsWithBotsOnly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	srv := newFillHub(make(chan time.Time), 20*time.Millisecond)
	defer srv.Close()
	a := dial2v2(ctx, t, srv, "10.2.2.1")
	readMsg(ctx, t, a, msgWaiting, headerLen)
	if _, _, err := a.Read(ctx); err == nil {
		t.Fatal("a lone 2v2 waiter got a match")
	}
}

func TestQueue2v2LeaverIsReplacedByBot(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tick := make(chan time.Time)
	srv := newFillHub(tick, 30*time.Millisecond)
	defer srv.Close()
	a := dial2v2(ctx, t, srv, "10.2.3.1")
	readMsg(ctx, t, a, msgWaiting, headerLen)
	b := dial2v2(ctx, t, srv, "10.2.3.2")
	readMsg(ctx, t, b, msgWaiting, headerLen)
	readMsg(ctx, t, a, msgWelcome, welcomeLen)
	readMsg(ctx, t, b, msgWelcome, welcomeLen)
	b.Close(websocket.StatusNormalClosure, "")
	time.Sleep(50 * time.Millisecond) // let the server see the leave
	tick <- time.Time{}
	readMsg(ctx, t, a, msgFrame, frameMsgLen) // the match goes on for a
}
