package main

import (
	"context"
	"io"
	"log"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/txny34/futenis/cmd/server/internal/auth/authtest"
	"github.com/txny34/futenis/cmd/server/internal/match"
	"github.com/txny34/futenis/cmd/server/internal/wire"
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
	readMsg(ctx, t, c1, wire.MsgWaiting, wire.HeaderLen)
	c2 = dial(ctx, t, srv)
	readWelcome(ctx, t, c1)
	readMsg(ctx, t, c2, wire.MsgWelcome, wire.WelcomeLen)
	seat := readMsg(ctx, t, c2, wire.MsgSeat, wire.HeaderLen+match.TokenLen)
	return c1, c2, string(seat[wire.HeaderLen:])
}

func TestRejoinResumesHeldSeat(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tick := make(chan time.Time)
	srv := newGraceHub(tick, time.Minute)
	defer srv.Close()
	c1, c2, token := startPair(ctx, t, srv)

	c2.CloseNow() // connection lost
	if a := readMsg(ctx, t, c1, wire.MsgPeerAway, wire.HeaderLen+8); wire.F64(a, 0) != 60 {
		t.Fatalf("away seconds %v, want 60", wire.F64(a, 0))
	}
	back := dialRejoin(ctx, t, srv, token)
	if w := readMsg(ctx, t, back, wire.MsgWelcome, wire.WelcomeLen); w[2] != 1 {
		t.Fatalf("rejoined slot %d, want 1", w[2])
	}
	if s := readMsg(ctx, t, back, wire.MsgSeat, wire.HeaderLen+match.TokenLen); string(s[wire.HeaderLen:]) != token {
		t.Fatal("rejoin token changed")
	}
	readMsg(ctx, t, back, wire.MsgStyles, wire.StylesMsgLen) // everyone's look, again
	readMsg(ctx, t, c1, wire.MsgPeerBack, wire.HeaderLen)
	tick <- time.Time{}
	readMsg(ctx, t, c1, wire.MsgFrame, wire.FrameLen)
	readMsg(ctx, t, back, wire.MsgFrame, wire.FrameLen)
}

func TestHoldExpiryEndsMatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newGraceHub(make(chan time.Time), 30*time.Millisecond)
	defer srv.Close()
	c1, c2, token := startPair(ctx, t, srv)
	c2.CloseNow()
	readMsg(ctx, t, c1, wire.MsgPeerAway, wire.HeaderLen+8)
	readMsg(ctx, t, c1, wire.MsgPeerLeft, wire.HeaderLen)
	late := dialRejoin(ctx, t, srv, token)
	if _, _, err := late.Read(ctx); websocket.CloseStatus(err) != wire.CloseGone {
		t.Fatalf("late rejoin: %v, want close %d", err, wire.CloseGone)
	}
}

func TestRejoinUnknownTokenRefused(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newGraceHub(make(chan time.Time), time.Minute)
	defer srv.Close()
	c := dialRejoin(ctx, t, srv, "0123456789abcdef0123456789abcdef")
	if _, _, err := c.Read(ctx); websocket.CloseStatus(err) != wire.CloseGone {
		t.Fatalf("unknown token: %v, want close %d", err, wire.CloseGone)
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
	for range match.MaxHolds {
		c2.CloseNow()
		readMsg(ctx, t, c1, wire.MsgPeerAway, wire.HeaderLen+8)
		c2 = dialRejoin(ctx, t, srv, token)
		readMsg(ctx, t, c2, wire.MsgWelcome, wire.WelcomeLen)
		readMsg(ctx, t, c2, wire.MsgSeat, wire.HeaderLen+match.TokenLen)
		readMsg(ctx, t, c1, wire.MsgPeerBack, wire.HeaderLen)
	}
	c2.CloseNow() // one drop too many: the seat is left for good
	readMsg(ctx, t, c1, wire.MsgPeerLeft, wire.HeaderLen)
}

func TestRejoinReplacesAConnectionNotSeenDropped(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tick := make(chan time.Time)
	srv := newGraceHub(tick, time.Minute)
	defer srv.Close()
	c1, c2, token := startPair(ctx, t, srv)
	back := dialRejoin(ctx, t, srv, token) // c2 still open: wifi -> 4G switch
	readMsg(ctx, t, back, wire.MsgWelcome, wire.WelcomeLen)
	readMsg(ctx, t, back, wire.MsgSeat, wire.HeaderLen+match.TokenLen)
	readMsg(ctx, t, back, wire.MsgStyles, wire.StylesMsgLen)
	expectClose(ctx, t, c2, wire.CloseReplace)
	tick <- time.Time{} // the match never paused nor ended
	readMsg(ctx, t, c1, wire.MsgFrame, wire.FrameLen)
	readMsg(ctx, t, back, wire.MsgFrame, wire.FrameLen)
}

func TestClosingTheTabLeaves(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newGraceHub(make(chan time.Time), time.Minute)
	defer srv.Close()
	c1, c2, _ := startPair(ctx, t, srv)
	_ = c2.Close(websocket.StatusGoingAway, "")
	readMsg(ctx, t, c1, wire.MsgPeerLeft, wire.HeaderLen)
}

// startRanked starts a ranked 1v1 between userA and userB; it returns A's rejoin token.
func startRanked(ctx context.Context, t *testing.T, srv *httptest.Server, iss *authtest.Issuer, net string) (a, b *websocket.Conn, token string) {
	t.Helper()
	a = dialRanked(ctx, t, srv, net+".1", tokenFor(t, iss, userA))
	readMsg(ctx, t, a, wire.MsgWaiting, wire.HeaderLen)
	b = dialRanked(ctx, t, srv, net+".2", tokenFor(t, iss, userB))
	readMsg(ctx, t, a, wire.MsgWelcome, wire.WelcomeLen)
	seat := readMsg(ctx, t, a, wire.MsgSeat, wire.HeaderLen+match.TokenLen)
	readWelcome(ctx, t, b)
	return a, b, string(seat[wire.HeaderLen:])
}

func TestRankedHeldSeatKeepsTheAccountBusy(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv, iss, _ := newRankedHub(t)
	a, b, token := startRanked(ctx, t, srv, iss, "10.4.1")
	a.CloseNow()
	readMsg(ctx, t, b, wire.MsgPeerAway, wire.HeaderLen+8)
	again := dialRanked(ctx, t, srv, "10.4.1.3", tokenFor(t, iss, userA)) // a second ranked match meanwhile
	expectClose(ctx, t, again, wire.CloseFull)
	back := dialRejoin(ctx, t, srv, token)
	readMsg(ctx, t, back, wire.MsgWelcome, wire.WelcomeLen)
}

func TestRankedBothDroppedIsNoContest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv, iss, store := newRankedHub(t)
	a, b, _ := startRanked(ctx, t, srv, iss, "10.4.2")
	a.CloseNow()
	readMsg(ctx, t, b, wire.MsgPeerAway, wire.HeaderLen+8)
	b.CloseNow() // nobody left to wait: the match ends without a result
	time.Sleep(100 * time.Millisecond)
	if got, _ := store.Load(ctx, "1v1", []string{userA, userB}); len(got) != 0 {
		t.Fatalf("ratings written %v, want none", got)
	}
	again := dialRanked(ctx, t, srv, "10.4.2.3", tokenFor(t, iss, userA)) // the account is free again
	readMsg(ctx, t, again, wire.MsgWaiting, wire.HeaderLen)
}

func TestRankedLeavingWhileRivalHeldIsAForfeit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv, iss, store := newRankedHub(t)
	a, b, _ := startRanked(ctx, t, srv, iss, "10.4.3")
	a.CloseNow() // A's connection blips
	readMsg(ctx, t, b, wire.MsgPeerAway, wire.HeaderLen+8)
	_ = b.Close(websocket.StatusNormalClosure, "") // B quits on purpose to dodge the loss
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got, _ := store.Load(ctx, "1v1", []string{userA, userB}); got[userB] != 0 && got[userB] < got[userA] {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	got, _ := store.Load(ctx, "1v1", []string{userA, userB})
	t.Fatalf("ratings %v: want B's forfeit recorded", got)
}
