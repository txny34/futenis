package ai

import (
	"testing"

	"github.com/txny34/futenis/sim"
)

// setUpBall returns a 2v2 rally where slot `passer` (team 0) just set the
// ball up for its partner: team 0 has used two touches, so the partner's
// touch is the one that must cross.
func setUpBall(passer int8) *sim.Match {
	cfg := sim.DefaultConfig()
	cfg.Mode = sim.Mode2v2
	m := sim.NewMatch(cfg)
	s := m.State()
	s.Phase = sim.PhaseRally
	s.Rally.LastTeam, s.Rally.LastPlayer, s.Rally.TeamTouches = 0, passer, 2
	s.Rally.ServeBoxPending = false
	s.Players[passer^1].Pos = sim.Vec3{X: -5, Z: 0.8}
	s.Players[passer].Pos = sim.Vec3{X: -8, Z: -3}
	s.Ball = sim.Ball{Pos: sim.Vec3{X: -5.5, Y: 3.5, Z: 0.6}, Tossed: true}
	m.Restore(s)
	return m
}

// After the partner's pass the bot finishes it: it strikes the ball before
// the point ends, and the shot crosses the net (it never passes back on the
// last touch).
func TestBotFinishesPartnersPass(t *testing.T) {
	for _, passer := range []int8{0, 1} {
		m := setUpBall(passer)
		finisher := int(passer ^ 1)
		bot := New(Hard, finisher)
		bot.Level.MissChance = 0 // a deliberate whiff is not what this test is about
		kicked := false
		for tick := 0; tick < 3*sim.TickRate && !kicked; tick++ {
			s := m.State()
			var in [sim.MaxPlayers]sim.PlayerInput
			in[finisher] = bot.Think(&s, finisher)
			for _, e := range m.Step(in) {
				if e.Kind == sim.EvPoint {
					t.Fatalf("passer %d: point ended before the partner bot struck the set", passer)
				}
				if e.Kind == sim.EvKick && int(e.Player) == finisher {
					kicked = true
					if e.Vel.X <= 0 {
						t.Fatalf("passer %d: finishing shot heads %+.2f in X, want toward the rival (+X)", passer, e.Vel.X)
					}
				}
			}
		}
		if !kicked {
			t.Fatalf("passer %d: the bot never struck its partner's set", passer)
		}
	}
}
