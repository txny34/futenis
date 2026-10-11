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
	"github.com/txny34/futenis/sim"
)

const (
	testIss = "https://proj.supabase.co/auth/v1"
	userA   = "8d1c7a3e-0000-4000-8000-00000000000a"
	userB   = "8d1c7a3e-0000-4000-8000-00000000000b"
)

func newRankedHub(t *testing.T) (*httptest.Server, *fakeIssuer, *memRatings) {
	t.Helper()
	iss := newFakeIssuer(t)
	store := newMemRatings()
	h := newHub([]string{"*"}, log.New(io.Discard, "", 0))
	h.proxied = true
	h.ticker = func() (<-chan time.Time, func()) { return make(chan time.Time), func() {} }
	h.auth = newVerifier(iss.srv.URL, testIss)
	h.ratings = store
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, iss, store
}

func dialRanked(ctx context.Context, t *testing.T, srv *httptest.Server, ip, token string) *websocket.Conn {
	t.Helper()
	c, _, err := websocket.Dial(ctx, "ws"+srv.URL[4:]+"/?queue=ranked1v1", &websocket.DialOptions{HTTPHeader: http.Header{"Fly-Client-IP": {ip}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.CloseNow() })
	if token != "" {
		b := make([]byte, headerLen+len(token))
		putHeader(b, msgAuth)
		copy(b[headerLen:], token)
		if err := c.Write(ctx, websocket.MessageBinary, b); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

func tokenFor(t *testing.T, iss *fakeIssuer, user string) string {
	c := goodClaims(testIss)
	c["sub"] = user
	return iss.token(t, c)
}

func TestRankedNeedsAValidToken(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv, _, _ := newRankedHub(t)
	c := dialRanked(ctx, t, srv, "10.3.0.1", "forged.token.here")
	expectClose(ctx, t, c, closeNoAuth)
}

func TestRankedPairsTwoAccountsNotOneTwice(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv, iss, _ := newRankedHub(t)
	a := dialRanked(ctx, t, srv, "10.3.1.1", tokenFor(t, iss, userA))
	readMsg(ctx, t, a, msgWaiting, headerLen)
	again := dialRanked(ctx, t, srv, "10.3.1.2", tokenFor(t, iss, userA)) // same account, other IP
	expectClose(ctx, t, again, closeFull)
	b := dialRanked(ctx, t, srv, "10.3.1.3", tokenFor(t, iss, userB))
	readWelcome(ctx, t, a)
	readWelcome(ctx, t, b)
}

func TestRankedDisabledWithoutConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newQueueHub()
	defer srv.Close()
	c := dialRanked(ctx, t, srv, "10.3.2.1", "x")
	expectClose(ctx, t, c, closeNoAuth)
}

func TestReportRankedStoresAndTellsEachPlayer(t *testing.T) {
	store := newMemRatings()
	_ = store.save(context.Background(), "1v1", []rankedRow{{ID: userA, Elo: 1000}, {ID: userB, Elo: 1000}})
	h := newHub([]string{"*"}, log.New(io.Discard, "", 0))
	h.ratings = store
	r := newRoom(sim.Mode1v1)
	pa, pb := &fakePeer{}, &fakePeer{}
	r.join(pa)
	r.join(pb)
	h.reportRanked("1v1", r, [sim.MaxPlayers]string{userA, userB}, 1)
	got, _ := store.load(context.Background(), "1v1", []string{userA, userB})
	if got[userA] != 984 || got[userB] != 1016 {
		t.Fatalf("stored %v", got)
	}
	last := pb.msgs[len(pb.msgs)-1]
	if last[0] != msgResult || f64(last, 0) != 1000 || f64(last, 1) != 1016 {
		t.Fatalf("winner result %v", last)
	}
}

func TestGameOverWinner(t *testing.T) {
	out := make([]float64, 2*sim.SnapLen)
	if _, over := gameOverWinner(out); over {
		t.Fatal("serve phase reported as over")
	}
	out[sim.SnapLen+sim.SnapPhase] = float64(sim.PhaseGameOver)
	out[sim.SnapLen+sim.SnapSets1] = 1
	if w, over := gameOverWinner(out); !over || w != 1 {
		t.Fatalf("w=%d over=%v", w, over)
	}
}

// Leaving a started ranked match is a forfeit: the leaver's team loses, once.
func TestRankedLeaverForfeits(t *testing.T) {
	r := newRoom(sim.Mode1v1)
	r.join(&fakePeer{})
	r.join(&fakePeer{})
	r.markRanked(0, claim{uid: userA}, "net-a")
	r.markRanked(1, claim{uid: userB}, "net-b")
	got := make(chan int, 2)
	r.onOver = func(w int, _ [sim.MaxPlayers]string) { got <- w }
	r.leave(0, nil, false) // userA quits
	select {
	case w := <-got:
		if w != 1 {
			t.Fatalf("winner %d, want the team that stayed (1)", w)
		}
	case <-time.After(time.Second):
		t.Fatal("no result reported for a ranked quit")
	}
	if r.claimReport() { // a later game over must report nothing
		t.Fatal("result could be reported twice")
	}
}

// A ranked 2v2 never hands a seat to a bot: a leaver ends the match.
func TestRanked2v2LeaverEndsMatch(t *testing.T) {
	r := newRoom(sim.Mode2v2)
	for i := 0; i < 4; i++ {
		r.join(&fakePeer{})
	}
	for i, u := range []string{userA, userB, userA + "c", userA + "d"} {
		r.markRanked(i, claim{uid: u}, u)
	}
	r.onOver = func(int, [sim.MaxPlayers]string) {}
	r.leave(2, nil, false)
	if !r.isClosed() {
		t.Fatal("ranked 2v2 kept going with a bot")
	}
}

// Two simultaneous connections of one account: the second is refused while the
// first is active, so nobody can be matched against themselves.
func TestRankedOneActiveConnectionPerAccount(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv, iss, _ := newRankedHub(t)
	tok := tokenFor(t, iss, userA)
	a := dialRanked(ctx, t, srv, "10.4.0.1", tok)
	b := dialRanked(ctx, t, srv, "10.4.0.2", tok) // same account, same instant
	got := 0
	for _, c := range []*websocket.Conn{a, b} {
		_, m, err := c.Read(ctx)
		if err == nil && m[0] == msgWaiting {
			got++
		} else if websocket.CloseStatus(err) != closeFull {
			t.Fatalf("unexpected: %v", err)
		}
	}
	if got != 1 {
		t.Fatalf("%d connections of one account were seated", got)
	}
}

func TestRankedRoomRefusesSameUserOrNetwork(t *testing.T) {
	r := newRoom(sim.Mode1v1)
	r.join(&fakePeer{})
	r.join(&fakePeer{})
	if !r.markRanked(0, claim{uid: userA}, "10.0.0.1") {
		t.Fatal("first seat refused")
	}
	if r.markRanked(1, claim{uid: userA}, "10.0.0.2") {
		t.Fatal("same account on both seats accepted")
	}
	if r.markRanked(1, claim{uid: userB}, "10.0.0.1") {
		t.Fatal("two accounts from one network accepted")
	}
}

func TestApplyEloCeiling(t *testing.T) {
	next := applyElo(map[string]int{"a": eloCeiling, "b": eloCeiling}, [2][]string{{"a"}, {"b"}}, 0)
	if next["a"] > eloCeiling {
		t.Fatalf("rating %d above the table's limit", next["a"])
	}
}
