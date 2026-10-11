package match

import (
	"testing"
	"time"

	"github.com/txny34/futenis/cmd/server/internal/peertest"
	"github.com/txny34/futenis/cmd/server/internal/ranked"
	"github.com/txny34/futenis/sim"
)

const (
	userA = "8d1c7a3e-0000-4000-8000-00000000000a"
	userB = "8d1c7a3e-0000-4000-8000-00000000000b"
)

// Leaving a started ranked match is a forfeit: the leaver's team loses, once.
func TestRankedLeaverForfeits(t *testing.T) {
	r := NewRoom(sim.Mode1v1, "test")
	r.Join(&peertest.Peer{})
	r.Join(&peertest.Peer{})
	r.MarkRanked(0, ranked.Claim{UID: userA}, "net-a")
	r.MarkRanked(1, ranked.Claim{UID: userB}, "net-b")
	got := make(chan int, 2)
	r.onOver = func(w int, _ [sim.MaxPlayers]string) { got <- w }
	r.Leave(0, nil, false) // userA quits
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
	r := NewRoom(sim.Mode2v2, "test")
	for i := 0; i < 4; i++ {
		r.Join(&peertest.Peer{})
	}
	for i, u := range []string{userA, userB, userA + "c", userA + "d"} {
		r.MarkRanked(i, ranked.Claim{UID: u}, u)
	}
	r.onOver = func(int, [sim.MaxPlayers]string) {}
	r.Leave(2, nil, false)
	if !r.IsClosed() {
		t.Fatal("ranked 2v2 kept going with a bot")
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

func TestPausedOnlyWhereNoBotPlays(t *testing.T) {
	held := func(mode sim.Mode, ranked bool) bool {
		r := NewRoom(mode, "test")
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
	if NewRoom(sim.Mode1v1, "test").pausedLocked() {
		t.Fatal("no held seat: not paused")
	}
}

func TestRequeueSkipsARoomThatStarted(t *testing.T) {
	r := NewRoom(sim.Mode1v1, "test")
	r.Join(&peertest.Peer{})
	r.Join(&peertest.Peer{}) // full: the match starts
	if p, _ := r.RequeueAlone(); p != nil || r.IsClosed() {
		t.Fatal("a started match must never be broken up")
	}
	lone := NewRoom(sim.Mode1v1, "test")
	lone.Join(&peertest.Peer{})
	if p, _ := lone.RequeueAlone(); p == nil || !lone.IsClosed() {
		t.Fatal("a lone waiter is moved and its room closed")
	}
	if slot, _, _ := lone.Join(&peertest.Peer{}); slot >= 0 {
		t.Fatal("a joiner racing the move must be refused (it retries)")
	}
}
