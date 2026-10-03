package main

import (
	"testing"

	"github.com/txny34/futenis/ai"
	"github.com/txny34/futenis/sim"
)

// humanLikeInput mimics the browser slot 0: momentary Strike clicks, no
// spin/sprint, aim deep (9.5) or short (1.5) -- serves often fault.
func humanLikeInput(h uint64) sim.PlayerInput {
	in := sim.PlayerInput{TargetX: 9.5, TargetZ: float64(int((h>>45)%9)-4) * 0.9}
	if (h>>33)%7 == 0 {
		in.Buttons = sim.BtnStrike
	}
	if (h>>40)%2 == 0 {
		in.TargetX = 1.5
	}
	return in
}

// Regression: in padel scoring one live ball must award exactly one point,
// always followed by the PhasePoint pause (bug: "lose the set in one point").
func TestHumanVsBotOnePointPerBall(t *testing.T) {
	ss := defaultSession()
	if err := ss.configure(int(sim.Mode1v1), int(ai.Normal), 5, 1); err != nil {
		t.Fatal(err)
	}
	out := make([]float64, outLen)
	h := uint64(7)
	prev := ss.match.State()
	for f := 0; f < 20000 && prev.Phase != sim.PhaseGameOver; f++ {
		h = h*6364136223846793005 + 1442695040888963407
		runFrame(ss.match, [sim.MaxPlayers]sim.PlayerInput{humanLikeInput(h)}, 1, out, &ss.drv)
		st := ss.match.State()
		won := st.Score != prev.Score || st.Games != prev.Games || st.Sets != prev.Sets
		if won && st.Phase != sim.PhasePoint && st.Phase != sim.PhaseGameOver {
			t.Fatalf("tick %d: score changed to %v/%v but phase=%d (ball still live)", st.Tick, st.Score, st.Games, st.Phase)
		}
		if won && prev.Phase == sim.PhasePoint {
			t.Fatalf("tick %d: scored again during the point pause", st.Tick)
		}
		prev = st
	}
	if prev.Phase != sim.PhaseGameOver {
		t.Fatal("match never finished")
	}
}
