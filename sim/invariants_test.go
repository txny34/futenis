package sim

import "testing"

// TestNewMatch_FirstServerSelectsInitialServingTeam covers Config.FirstServer
// (code-review hardening: a bot-vs-bot tournament harness alternating this
// per match is how the per-game serve alternation inside a match -- which
// only balances things WITHIN one match -- doesn't still hand the same team
// a first-serve edge across many independently-started matches).
func TestNewMatch_FirstServerSelectsInitialServingTeam(t *testing.T) {
	cases := []struct {
		firstServer int8
		want        int8
	}{
		{0, 0},
		{1, 1},
	}
	for _, c := range cases {
		cfg := DefaultConfig()
		cfg.FirstServer = c.firstServer
		cfg.Sanitize()
		m := NewMatch(cfg)
		s := m.State()
		if s.Server != c.want {
			t.Errorf("FirstServer=%d: Server got %d, want %d", c.firstServer, s.Server, c.want)
		}
		if s.Players[s.Rally.ServerPlayer].Team != s.Server {
			t.Errorf("FirstServer=%d: ServerPlayer=%d has Team=%d, want == Server=%d", c.firstServer, s.Rally.ServerPlayer, s.Players[s.Rally.ServerPlayer].Team, s.Server)
		}
	}
}

func TestConfigSanitize_FirstServerOutOfRangeFallsBackTo0(t *testing.T) {
	c := DefaultConfig()
	c.FirstServer = 5
	c.Sanitize()
	if c.FirstServer != 0 {
		t.Fatalf("FirstServer: got %d, want 0 fallback", c.FirstServer)
	}
	c.FirstServer = -1
	c.Sanitize()
	if c.FirstServer != 0 {
		t.Fatalf("FirstServer: got %d, want 0 fallback", c.FirstServer)
	}
}

// TestInvariant_ServerPlayerTeamMatchesServer drives several short scripted
// matches (every Mode x ScoringMode x FirstServer combination) and asserts,
// every tick, that Rally.ServerPlayer always belongs to Team == Server --
// the exact invariant the judgeServeVolley bug (early-receiver-touch
// mis-dispatch into serveFault with the wrong team) used to violate.
func TestInvariant_ServerPlayerTeamMatchesServer(t *testing.T) {
	const ticks = 8000
	modes := []Mode{Mode1v1, Mode2v2}
	scoringModes := []ScoringMode{ScoringSimple, ScoringPadel}
	firstServers := []int8{0, 1}

	for _, mode := range modes {
		for _, sm := range scoringModes {
			for _, fs := range firstServers {
				cfg := DefaultConfig()
				cfg.Mode = mode
				cfg.ScoringMode = sm
				cfg.FirstServer = fs
				cfg.Sanitize()

				m := NewMatch(cfg)
				gens := make([]*lcg, MaxPlayers)
				for i := range gens {
					gens[i] = newLCG(uint64(mode)*7919 + uint64(sm)*104729 + uint64(fs)*1299721 + uint64(i) + 1)
				}

				for tick := 0; tick < ticks; tick++ {
					var inputs [MaxPlayers]PlayerInput
					s := m.State()
					for i := int8(0); i < s.NumPlayers; i++ {
						inputs[i] = scriptedInput(gens[i])
					}
					m.Step(inputs)

					st := m.State()
					if st.Phase != PhaseServe && st.Phase != PhaseRally {
						if st.Phase == PhaseGameOver {
							break
						}
						continue
					}
					sp := st.Rally.ServerPlayer
					if sp < 0 || sp >= st.NumPlayers {
						continue
					}
					if st.Players[sp].Team != st.Server {
						t.Fatalf("mode=%v scoring=%v firstServer=%d tick=%d: ServerPlayer=%d has Team=%d, want == Server=%d",
							mode, sm, fs, tick, sp, st.Players[sp].Team, st.Server)
					}
					if st.Phase == PhaseGameOver {
						break
					}
				}
			}
		}
	}
}
