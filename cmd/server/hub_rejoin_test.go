package main

import (
	"context"
	"io"
	"log"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/txny34/futenis/sim"
)

func newGraceHub(tick chan time.Time, grace time.Duration) *httptest.Server {
	h := newHub([]string{"*"}, log.New(io.Discard, "", 0))
	h.ticker = func() (<-chan time.Time, func()) { return tick, func() {} }
	h.grace = grace
	return httptest.NewServer(h)
}

func dialRejoin(ctx context.Context, t *testing.T, srv *httptest.Server, token string) *websocket.Conn {
	t.Helper()
	c, _, err := websocket.Dial(ctx, "ws"+srv.URL[4:]+"/?rejoin="+token, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.CloseNow() })
	return c
}

// startPair starts a 1v1 and returns host, guest and the guest's rejoin token.
func startPair(ctx context.Context, t *testing.T, srv *httptest.Server) (c1, c2 *websocket.Conn, token string) {
	t.Helper()
	c1 = dial(ctx, t, srv)
	readMsg(ctx, t, c1, msgWaiting, headerLen)
	c2 = dial(ctx, t, srv)
	readWelcome(ctx, t, c1)
	readMsg(ctx, t, c2, msgWelcome, welcomeLen)
	seat := readMsg(ctx, t, c2, msgSeat, headerLen+tokenLen)
	return c1, c2, string(seat[headerLen:])
}

func TestRejoinResumesHeldSeat(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tick := make(chan time.Time)
	srv := newGraceHub(tick, time.Minute)
	defer srv.Close()
	c1, c2, token := startPair(ctx, t, srv)

	c2.CloseNow() // connection lost
	if a := readMsg(ctx, t, c1, msgPeerAway, headerLen+8); f64(a, 0) != 60 {
		t.Fatalf("away seconds %v, want 60", f64(a, 0))
	}
	back := dialRejoin(ctx, t, srv, token)
	if w := readMsg(ctx, t, back, msgWelcome, welcomeLen); w[2] != 1 {
		t.Fatalf("rejoined slot %d, want 1", w[2])
	}
	if s := readMsg(ctx, t, back, msgSeat, headerLen+tokenLen); string(s[headerLen:]) != token {
		t.Fatal("rejoin token changed")
	}
	readMsg(ctx, t, c1, msgPeerBack, headerLen)
	tick <- time.Time{}
	readMsg(ctx, t, c1, msgFrame, frameMsgLen)
	readMsg(ctx, t, back, msgFrame, frameMsgLen)
}

func TestHoldExpiryEndsMatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newGraceHub(make(chan time.Time), 30*time.Millisecond)
	defer srv.Close()
	c1, c2, token := startPair(ctx, t, srv)
	c2.CloseNow()
	readMsg(ctx, t, c1, msgPeerAway, headerLen+8)
	readMsg(ctx, t, c1, msgPeerLeft, headerLen)
	late := dialRejoin(ctx, t, srv, token)
	if _, _, err := late.Read(ctx); websocket.CloseStatus(err) != closeGone {
		t.Fatalf("late rejoin: %v, want close %d", err, closeGone)
	}
}

func TestRejoinUnknownTokenRefused(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newGraceHub(make(chan time.Time), time.Minute)
	defer srv.Close()
	c := dialRejoin(ctx, t, srv, "0123456789abcdef0123456789abcdef")
	if _, _, err := c.Read(ctx); websocket.CloseStatus(err) != closeGone {
		t.Fatalf("unknown token: %v, want close %d", err, closeGone)
	}
	res, err := srv.Client().Get(srv.URL + "/?rejoin=nothex")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("malformed token: status %d, want 400", res.StatusCode)
	}
}

func TestHoldsAreCapped(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newGraceHub(make(chan time.Time), time.Minute)
	defer srv.Close()
	c1, c2, token := startPair(ctx, t, srv)
	for range maxRejoins {
		c2.CloseNow()
		readMsg(ctx, t, c1, msgPeerAway, headerLen+8)
		c2 = dialRejoin(ctx, t, srv, token)
		readMsg(ctx, t, c2, msgWelcome, welcomeLen)
		readMsg(ctx, t, c2, msgSeat, headerLen+tokenLen)
		readMsg(ctx, t, c1, msgPeerBack, headerLen)
	}
	c2.CloseNow() // one drop too many: the seat is left for good
	readMsg(ctx, t, c1, msgPeerLeft, headerLen)
}

func TestPausedOnlyWhereNoBotPlays(t *testing.T) {
	held := func(mode sim.Mode, ranked bool) bool {
		r := newRoom(mode)
		r.ranked = ranked
		r.away[1] = time.NewTimer(time.Hour)
		defer r.away[1].Stop()
		return r.pausedLocked()
	}
	if !held(sim.Mode1v1, false) || !held(sim.Mode2v2, true) {
		t.Fatal("1v1 and ranked must pause while a seat is held")
	}
	if held(sim.Mode2v2, false) {
		t.Fatal("casual 2v2 goes on with the bot")
	}
	if newRoom(sim.Mode1v1).pausedLocked() {
		t.Fatal("no held seat: not paused")
	}
}
