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
	"github.com/txny34/futenis/cmd/server/internal/auth"
	"github.com/txny34/futenis/cmd/server/internal/auth/authtest"
	"github.com/txny34/futenis/cmd/server/internal/match"
	"github.com/txny34/futenis/cmd/server/internal/peertest"
	"github.com/txny34/futenis/cmd/server/internal/ranked"
	"github.com/txny34/futenis/cmd/server/internal/wire"
	"github.com/txny34/futenis/sim"
)

const (
	testIss = "https://proj.supabase.co/auth/v1"
	userA   = "8d1c7a3e-0000-4000-8000-00000000000a"
	userB   = "8d1c7a3e-0000-4000-8000-00000000000b"
)

func newRankedHub(t *testing.T) (*httptest.Server, *authtest.Issuer, *ranked.Mem) {
	t.Helper()
	iss := authtest.NewIssuer(t)
	store := ranked.NewMem()
	h := newHub([]string{"*"}, log.New(io.Discard, "", 0))
	h.proxied = true
	h.ticker = func() (<-chan time.Time, func()) { return make(chan time.Time), func() {} }
	h.auth = auth.NewVerifier(iss.Srv.URL, testIss)
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
		b := make([]byte, wire.HeaderLen+len(token))
		wire.PutHeader(b, wire.MsgAuth)
		copy(b[wire.HeaderLen:], token)
		if err := c.Write(ctx, websocket.MessageBinary, b); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

func tokenFor(t *testing.T, iss *authtest.Issuer, user string) string {
	c := authtest.GoodClaims(testIss)
	c["sub"] = user
	return iss.Token(t, c)
}

func TestRankedNeedsAValidToken(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv, _, _ := newRankedHub(t)
	c := dialRanked(ctx, t, srv, "10.3.0.1", "forged.token.here")
	expectClose(ctx, t, c, wire.CloseNoAuth)
}

func TestRankedPairsTwoAccountsNotOneTwice(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv, iss, _ := newRankedHub(t)
	a := dialRanked(ctx, t, srv, "10.3.1.1", tokenFor(t, iss, userA))
	readMsg(ctx, t, a, wire.MsgWaiting, wire.HeaderLen)
	again := dialRanked(ctx, t, srv, "10.3.1.2", tokenFor(t, iss, userA)) // same account, other IP
	expectClose(ctx, t, again, wire.CloseFull)
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
	expectClose(ctx, t, c, wire.CloseNoAuth)
}

func TestReportRankedStoresAndTellsEachPlayer(t *testing.T) {
	store := ranked.NewMem()
	_ = store.Save(context.Background(), "1v1", []ranked.Row{{ID: userA, Elo: 1000}, {ID: userB, Elo: 1000}})
	h := newHub([]string{"*"}, log.New(io.Discard, "", 0))
	h.ratings = store
	r := match.NewRoom(sim.Mode1v1, "test")
	pa, pb := &peertest.Peer{}, &peertest.Peer{}
	r.Join(pa)
	r.Join(pb)
	h.reportRanked("1v1", r, [sim.MaxPlayers]string{userA, userB}, 1)
	got, _ := store.Load(context.Background(), "1v1", []string{userA, userB})
	if got[userA] != 984 || got[userB] != 1016 {
		t.Fatalf("stored %v", got)
	}
	msgs, _ := pb.Snapshot()
	last := msgs[len(msgs)-1]
	if last[0] != wire.MsgResult || wire.F64(last, 0) != 1000 || wire.F64(last, 1) != 1016 {
		t.Fatalf("winner result %v", last)
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
		if err == nil && m[0] == wire.MsgWaiting {
			got++
		} else if websocket.CloseStatus(err) != wire.CloseFull {
			t.Fatalf("unexpected: %v", err)
		}
	}
	if got != 1 {
		t.Fatalf("%d connections of one account were seated", got)
	}
}

func TestRankedRoomRefusesSameUserOrNetwork(t *testing.T) {
	r := match.NewRoom(sim.Mode1v1, "test")
	r.Join(&peertest.Peer{})
	r.Join(&peertest.Peer{})
	if !r.MarkRanked(0, ranked.Claim{UID: userA}, "10.0.0.1") {
		t.Fatal("first seat refused")
	}
	if r.MarkRanked(1, ranked.Claim{UID: userA}, "10.0.0.2") {
		t.Fatal("same account on both seats accepted")
	}
	if r.MarkRanked(1, ranked.Claim{UID: userB}, "10.0.0.1") {
		t.Fatal("two accounts from one network accepted")
	}
}

// JoinRanked checks and records the account under the seat's lock: a second
// player of the same account or network is refused before taking a seat, so
// the room keeps waiting for a real rival.
func TestJoinRankedRefusesSameUserOrNetwork(t *testing.T) {
	cases := []struct {
		name, user, net string
		conflict        bool
	}{
		{"other account and network", userB, "10.0.0.2", false},
		{"same account", userA, "10.0.0.2", true},
		{"same network", userB, "10.0.0.1", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := match.NewRoom(sim.Mode1v1, "test")
			if slot, _, _, c := r.JoinRanked(&peertest.Peer{}, match.Ranked{Claim: ranked.Claim{UID: userA}, Network: "10.0.0.1"}); slot != 0 || c {
				t.Fatalf("first seat: slot %d conflict %v", slot, c)
			}
			slot, start, _, c := r.JoinRanked(&peertest.Peer{}, match.Ranked{Claim: ranked.Claim{UID: tc.user}, Network: tc.net})
			if c != tc.conflict || (slot >= 0) == tc.conflict || start == tc.conflict {
				t.Fatalf("slot %d start %v conflict %v, want conflict %v", slot, start, c, tc.conflict)
			}
		})
	}
}

func TestApplyEloCeiling(t *testing.T) {
	next := ranked.ApplyElo(map[string]int{"a": ranked.EloCeiling, "b": ranked.EloCeiling}, [2][]string{{"a"}, {"b"}}, 0)
	if next["a"] > ranked.EloCeiling {
		t.Fatalf("rating %d above the table's limit", next["a"])
	}
}
