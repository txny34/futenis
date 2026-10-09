package game

import (
	"testing"

	"github.com/txny34/futenis/ai"
	"github.com/txny34/futenis/sim"
)

// realFrame builds a non-trivial frame (real positions, score, events) by
// running an actual 1v1 session for a while.
func realFrame(t *testing.T) []float64 {
	t.Helper()
	g := NewSession()
	if err := g.Configure(int(sim.Mode1v1), int(ai.Normal), 5, 0b11); err != nil {
		t.Fatalf("configure: %v", err)
	}
	in := [sim.MaxPlayers]sim.PlayerInput{
		{MoveX: 1, Buttons: sim.BtnStrike},
	}
	out := make([]float64, OutLen)
	for i := 0; i < 16; i++ {
		g.Run(in, 15, out)
	}
	return out
}

func TestRotateOut1v1Involution(t *testing.T) {
	out := realFrame(t)
	a := make([]float64, OutLen)
	b := make([]float64, OutLen)
	RotateOut1v1(a, out)
	RotateOut1v1(b, a)
	for i := range out {
		if b[i] != out[i] {
			t.Fatalf("index %d: got %v want %v", i, b[i], out[i])
		}
	}
}

func TestRotateOut1v1Fields(t *testing.T) {
	src := make([]float64, OutLen)
	src[outCurr+sim.SnapBallX] = 3
	src[outCurr+sim.SnapP0X] = -4
	src[outCurr+sim.SnapP1X] = 5
	src[outCurr+sim.SnapScore0] = 2
	src[outCurr+sim.SnapServer] = 0
	src[outCurr+sim.SnapLastTeam] = -1
	src[outCurr+sim.SnapServerPlayer] = 0

	src[outEventCount] = 1
	ev := src[outEvents:]
	ev[2] = 2 // posX
	ev[5] = 1 // side
	ev[6] = 0 // player

	dst := make([]float64, OutLen)
	RotateOut1v1(dst, src)

	if got := dst[outCurr+sim.SnapBallX]; got != -3 {
		t.Errorf("SnapBallX: got %v want -3", got)
	}
	if got := dst[outCurr+sim.SnapP0X]; got != -5 {
		t.Errorf("SnapP0X: got %v want -5 (negated P1X)", got)
	}
	if got := dst[outCurr+sim.SnapP1X]; got != 4 {
		t.Errorf("SnapP1X: got %v want 4 (negated P0X)", got)
	}
	if got := dst[outCurr+sim.SnapScore0]; got != 0 {
		t.Errorf("SnapScore0: got %v want 0 (swapped from Score1)", got)
	}
	if got := dst[outCurr+sim.SnapScore1]; got != 2 {
		t.Errorf("SnapScore1: got %v want 2 (swapped from Score0)", got)
	}
	if got := dst[outCurr+sim.SnapServer]; got != 1 {
		t.Errorf("SnapServer: got %v want 1 (flipped from 0)", got)
	}
	if got := dst[outCurr+sim.SnapLastTeam]; got != -1 {
		t.Errorf("SnapLastTeam: got %v want -1 (kept)", got)
	}
	if got := dst[outCurr+sim.SnapServerPlayer]; got != 1 {
		t.Errorf("SnapServerPlayer: got %v want 1 (flipped from 0)", got)
	}

	devt := dst[outEvents:]
	if got := devt[2]; got != -2 {
		t.Errorf("event posX: got %v want -2", got)
	}
	if got := devt[5]; got != 0 {
		t.Errorf("event side: got %v want 0 (flipped from 1)", got)
	}
	if got := devt[6]; got != 1 {
		t.Errorf("event player: got %v want 1 (flipped from 0)", got)
	}
}

func TestRotateInput(t *testing.T) {
	in := sim.PlayerInput{MoveX: 1, MoveZ: 2, TargetX: 3, TargetZ: 4, Buttons: sim.BtnStrike, Spin: 1}
	got := RotateInput(in)

	if got.MoveX != -1 || got.MoveZ != -2 {
		t.Errorf("Move: got (%v,%v) want (-1,-2)", got.MoveX, got.MoveZ)
	}
	if got.TargetX != -3 || got.TargetZ != -4 {
		t.Errorf("Target: got (%v,%v) want (-3,-4)", got.TargetX, got.TargetZ)
	}
	if got.Buttons != sim.BtnStrike {
		t.Errorf("Buttons: got %v want unchanged", got.Buttons)
	}
	if got.Spin != 1 {
		t.Errorf("Spin: got %v want unchanged", got.Spin)
	}
}

func TestRotateOut1v1NoAllocs(t *testing.T) {
	src := realFrame(t)
	dst := make([]float64, OutLen)
	n := testing.AllocsPerRun(100, func() {
		RotateOut1v1(dst, src)
	})
	if n != 0 {
		t.Fatalf("RotateOut1v1 allocs: got %v want 0", n)
	}
}
