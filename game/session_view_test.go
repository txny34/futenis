package game

import (
	"slices"
	"testing"

	"github.com/txny34/futenis/ai"
	"github.com/txny34/futenis/sim"
)

func TestConfigureFirstForcesServer(t *testing.T) {
	g := NewSession()
	for _, first := range []int{1, 0, 1} {
		if err := g.ConfigureFirst(int(sim.Mode1v1), int(ai.Normal), 5, 1, first); err != nil {
			t.Fatal(err)
		}
		if g.FirstServer() != first {
			t.Fatalf("FirstServer %d want %d", g.FirstServer(), first)
		}
	}
	if err := g.ConfigureFirst(int(sim.Mode1v1), int(ai.Normal), 5, 1, 2); err == nil {
		t.Fatal("firstServer 2 accepted")
	}
}

// Run with a view equals ViewOut of the server-space frame.
func TestSetViewRotatesRun(t *testing.T) {
	a, b := NewSession(), NewSession()
	for _, g := range []*Session{a, b} {
		if err := g.ConfigureFirst(int(sim.Mode2v2), int(ai.Normal), 5, 0b0101, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.SetView(2, true); err != nil {
		t.Fatal(err)
	}
	var in [sim.MaxPlayers]sim.PlayerInput
	in[0].MoveX, in[2].MoveZ = 1, -1
	outA, outB := make([]float64, OutLen), make([]float64, OutLen)
	want, scratch := make([]float64, OutLen), make([]float64, OutLen)
	for f := 0; f < 120; f++ {
		a.Run(in, 2, outA)
		b.Run(in, 2, outB)
	}
	ViewOut(want, outA, scratch, 2, true)
	if !slices.Equal(want, outB) {
		t.Fatal("SetView output differs from ViewOut of the server frame")
	}
	if err := b.SetView(4, true); err == nil {
		t.Fatal("seat 4 accepted")
	}
}

func TestRunWithViewAllocsNothing(t *testing.T) {
	g := NewSession()
	if err := g.ConfigureFirst(int(sim.Mode1v1), int(ai.Normal), 5, 1, 0); err != nil {
		t.Fatal(err)
	}
	if err := g.SetView(1, false); err != nil {
		t.Fatal(err)
	}
	var in [sim.MaxPlayers]sim.PlayerInput
	out := make([]float64, OutLen)
	if n := testing.AllocsPerRun(100, func() { g.Run(in, 2, out) }); n != 0 {
		t.Fatalf("Run allocates %v", n)
	}
}
