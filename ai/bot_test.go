package ai

import (
	"testing"

	"github.com/txny34/futenis/sim"
)

// runInputs plays n ticks of a bot-only match and records slot 0's inputs.
func runInputs(mode sim.Mode, d Difficulty, n int) []sim.PlayerInput {
	cfg := sim.DefaultConfig()
	cfg.Mode = mode
	m := sim.NewMatch(cfg)
	var bots [sim.MaxPlayers]Bot
	for i := range bots {
		bots[i] = New(d, i)
	}
	out := make([]sim.PlayerInput, 0, n)
	for t := 0; t < n; t++ {
		s := m.State()
		var in [sim.MaxPlayers]sim.PlayerInput
		for i := 0; i < int(s.NumPlayers); i++ {
			in[i] = bots[i].Think(&s, i)
		}
		out = append(out, in[0])
		m.Step(in)
	}
	return out
}

func TestThinkDeterministic(t *testing.T) {
	for _, mode := range []sim.Mode{sim.Mode1v1, sim.Mode2v2, sim.ModePlayground} {
		for d := Easy; d <= Pro; d++ {
			a, b := runInputs(mode, d, 1200), runInputs(mode, d, 1200)
			for i := range a {
				if a[i] != b[i] {
					t.Fatalf("mode %d %v: tick %d differs: %+v vs %+v", mode, d, i, a[i], b[i])
				}
			}
		}
	}
}

func TestThinkZeroAllocs(t *testing.T) {
	m := sim.NewMatch(sim.DefaultConfig())
	bots := [2]Bot{New(Pro, 0), New(Easy, 1)}
	allocs := testing.AllocsPerRun(2000, func() {
		s := m.State()
		var in [sim.MaxPlayers]sim.PlayerInput
		in[0] = bots[0].Think(&s, 0)
		in[1] = bots[1].Think(&s, 1)
		m.Step(in)
	})
	if allocs != 0 {
		t.Fatalf("Think+Step allocated %v per tick", allocs)
	}
}

func TestThinkOutOfRangeSlotIsIdle(t *testing.T) {
	m := sim.NewMatch(sim.DefaultConfig())
	s := m.State()
	b := New(Pro, 3)
	if in := b.Think(&s, 3); in != (sim.PlayerInput{}) {
		t.Fatalf("inactive slot got %+v", in)
	}
	if in := b.Think(&s, -1); in != (sim.PlayerInput{}) {
		t.Fatalf("negative slot got %+v", in)
	}
}

func TestServerReachesBallAndServes(t *testing.T) {
	r := playMatch(sim.Mode1v1, [sim.MaxPlayers]Difficulty{Easy, Easy}, 0)
	if r.kicks == 0 {
		t.Fatal("bots never put the ball in play")
	}
}

func TestBotMatchesAlwaysFinish(t *testing.T) {
	for _, mode := range []sim.Mode{sim.Mode1v1, sim.Mode2v2} {
		for d := Easy; d <= Pro; d++ {
			for seed := uint64(0); seed < 2; seed++ {
				r := playMatch(mode, [sim.MaxPlayers]Difficulty{d, Pro - d, d, Pro - d}, seed)
				if r.winner < 0 {
					t.Fatalf("mode %d %v seed %d hit the tick cap", mode, d, seed)
				}
			}
		}
	}
}

func TestLevelOfClampsAndString(t *testing.T) {
	if LevelOf(99) != LevelOf(Pro) {
		t.Fatal("out of range difficulty must clamp to Pro")
	}
	if Difficulty(99).String() == "" || Easy.String() != "Easy" {
		t.Fatal("String")
	}
}
