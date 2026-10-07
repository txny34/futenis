package sim

import (
	"math"
	"testing"
)

// TestLobby_Soak_Determinism: two lobbies, same scripted inputs for 10 000
// ticks, identical states every tick; bounds hold throughout.
func TestLobby_Soak_Determinism(t *testing.T) {
	a, b := newLobby(t), newLobby(t)
	ga, gb := newLCG(42), newLCG(42)
	limX := a.State().Cfg.LobbyHalfX - PlayerRadius
	limZ := a.State().Cfg.LobbyHalfZ - PlayerRadius
	starts := 0
	for i := 0; i < 10000; i++ {
		ina, inb := scriptedInput(ga), scriptedInput(gb)
		if i%600 < 300 { // periodically park on a pad so the hold path runs
			pad := LobbyPads[(i/600)%LobbyPadCount]
			s := a.State().Players[0].Pos
			ina.MoveX, ina.MoveZ = clamp(pad.X-s.X, -1, 1), clamp(pad.Z-s.Z, -1, 1)
			inb.MoveX, inb.MoveZ = ina.MoveX, ina.MoveZ
			ina.Buttons, inb.Buttons = 0, 0
		}
		starts += countKind(a.Step([MaxPlayers]PlayerInput{ina}), EvPadStart)
		b.Step([MaxPlayers]PlayerInput{inb})
		sa, sb := a.State(), b.State()
		if sa != sb {
			t.Fatalf("tick %d: states diverged", i)
		}
		p := sa.Players[0].Pos
		if math.Abs(p.X) > limX || math.Abs(p.Z) > limZ || math.IsNaN(p.X) || math.IsNaN(p.Z) {
			t.Fatalf("tick %d: player out of bounds %+v", i, p)
		}
	}
	if starts == 0 {
		t.Fatal("soak never exercised EvPadStart")
	}
}
