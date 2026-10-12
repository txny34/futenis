package ai

import (
	"testing"

	"github.com/txny34/futenis/sim"
)

// perfectRunTicks is the simulated time of each perfect-rate run (90 s).
const perfectRunTicks = 90 * sim.TickRate

// rallyPerfectRate plays d-vs-d (1v1 and 2v2, padel scoring, restarting at
// game over) and returns the share of PERFECT rally kicks. The serve and
// its return are excluded: both are played side-on by design (the body
// capsule rules out a close stand-off there), so they are never PERFECT.
// It calibrates shot skill, so the bots do not pass (2v2 team play and the
// smashes it sets up are covered by pass_test.go).
func rallyPerfectRate(d Difficulty) float64 {
	kicks, perfect := 0, 0
	for _, mode := range []sim.Mode{sim.Mode1v1, sim.Mode2v2} {
		cfg := sim.DefaultConfig()
		cfg.Mode, cfg.ScoringMode = mode, sim.ScoringPadel
		m := sim.NewMatch(cfg)
		var bots [sim.MaxPlayers]Bot
		for i := range bots {
			bots[i] = New(d, i)
			bots[i].Level.PassChance = 0
		}
		for t := 0; t < perfectRunTicks; t++ {
			s := m.State()
			if s.Phase == sim.PhaseGameOver {
				m = sim.NewMatch(cfg)
				continue
			}
			var in [sim.MaxPlayers]sim.PlayerInput
			for i := 0; i < int(s.NumPlayers); i++ {
				in[i] = bots[i].Think(&s, i)
			}
			for _, e := range m.Step(in) {
				if e.Kind == sim.EvKick && s.Rally.ShotCount >= 2 {
					kicks++
					if e.Quality == sim.QualPerfect {
						perfect++
					}
				}
			}
		}
	}
	return float64(perfect) / float64(max(kicks, 1))
}

func TestPerfectRateScalesWithDifficulty(t *testing.T) {
	prev := -1.0
	for d := Easy; d <= Pro; d++ {
		r := rallyPerfectRate(d)
		t.Logf("%-6v perfect rally kicks %.1f%%", d, 100*r)
		if r <= prev {
			t.Errorf("%v perfect rate %.3f does not beat the easier level's %.3f", d, r, prev)
		}
		prev = r
	}
	if prev < 0.40 {
		t.Errorf("Pro perfect rate %.3f, want >= 0.40", prev)
	}
}
