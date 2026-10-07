package ai

import (
	"testing"

	"github.com/txny34/futenis/sim"
)

// The bot never presses Lob while serving (padel has no lob serve).
func TestBotNeverLobsOnServe(t *testing.T) {
	for d := Easy; d <= Pro; d++ {
		cfg := sim.DefaultConfig()
		cfg.ScoringMode = sim.ScoringPadel
		m := sim.NewMatch(cfg)
		bots := [2]Bot{New(d, 0), New(d, 1)}
		for tick := 0; tick < 60*sim.TickRate; tick++ {
			s := m.State()
			if s.Phase == sim.PhaseGameOver {
				break
			}
			var in [sim.MaxPlayers]sim.PlayerInput
			for i := range bots {
				in[i] = bots[i].Think(&s, i)
			}
			srv := s.Rally.ServerPlayer
			if s.Phase == sim.PhaseServe && in[srv].Buttons&sim.BtnLob != 0 {
				t.Fatalf("difficulty %d tick %d: server pressed Lob on the serve", d, tick)
			}
			m.Step(in)
		}
	}
}
