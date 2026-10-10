package game

import (
	"testing"

	"github.com/txny34/futenis/sim"
)

// Every 2v2 client renders itself as slot 0 / team 0: ViewOut(slot) must put
// that slot's player block in slot 0, on team 0's side of the court.
func TestViewOutPutsEachSlotAtZero(t *testing.T) {
	src := make([]float64, OutLen)
	pos := [sim.MaxPlayers]int{sim.SnapP0X, sim.SnapP1X, sim.SnapP2X, sim.SnapP3X}
	xs := [sim.MaxPlayers]float64{-4, 4, -6, 6} // team 0 at X<0, team 1 at X>0
	for i, p := range pos {
		src[outCurr+p] = xs[i]
		src[outCurr+p+1] = float64(10 + i) // Y tags the original slot
	}
	dst := make([]float64, OutLen)
	scratch := make([]float64, OutLen)
	for slot := 0; slot < sim.MaxPlayers; slot++ {
		ViewOut(dst, src, scratch, slot)
		if tag := dst[outCurr+sim.SnapP0Y]; tag != float64(10+slot) {
			t.Fatalf("slot %d: slot 0 shows player %v", slot, tag-10)
		}
		if x := dst[outCurr+sim.SnapP0X]; x >= 0 {
			t.Fatalf("slot %d: own player at X=%v, want team 0's side (X<0)", slot, x)
		}
		partner := dst[outCurr+sim.SnapP2Y] - 10
		if int(partner)%2 != slot%2 {
			t.Fatalf("slot %d: slot 2 shows player %v, not a teammate", slot, partner)
		}
	}
}

func TestSwapPartnersInvolution(t *testing.T) {
	out := realFrame(t)
	a := make([]float64, OutLen)
	b := make([]float64, OutLen)
	SwapPartnersOut(a, out)
	SwapPartnersOut(b, a)
	for i := range out {
		if b[i] != out[i] {
			t.Fatalf("index %d: got %v want %v", i, b[i], out[i])
		}
	}
}

func TestViewInput(t *testing.T) {
	in := sim.PlayerInput{MoveX: 1, MoveZ: -1, TargetX: 3, TargetZ: 2}
	for slot, wantX := range map[int]float64{0: 1, 1: -1, 2: 1, 3: -1} {
		if got := ViewInput(in, slot); got.MoveX != wantX {
			t.Errorf("slot %d: MoveX %v want %v", slot, got.MoveX, wantX)
		}
	}
}

func TestViewOutNoAllocs(t *testing.T) {
	src := realFrame(t)
	dst := make([]float64, OutLen)
	scratch := make([]float64, OutLen)
	if n := testing.AllocsPerRun(50, func() { ViewOut(dst, src, scratch, 3) }); n != 0 {
		t.Fatalf("ViewOut allocates %v per call", n)
	}
}
