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

// Bots build the play in 2v2: over a match some possessions are a pass
// (two touches) and some the full A-B-A (pass, pass back, finish).
func TestBotsPlayPassesInDoubles(t *testing.T) {
	maxRun, passes, aba := 0, 0, 0
	for seed := uint64(1); seed <= 4; seed++ {
		r, p, a := passRuns(seed)
		maxRun, passes, aba = max(maxRun, r), passes+p, aba+a
	}
	if passes == 0 || aba == 0 {
		t.Fatalf("passes=%d A-B-A=%d (longest team run %d), want both > 0", passes, aba, maxRun)
	}
	if maxRun > 3 {
		t.Fatalf("a team touched the ball %d times in a row", maxRun)
	}
}

// passRuns plays one Hard 2v2 bot match and counts possessions with two
// (a pass) and three (A-B-A) touches, plus the longest team run.
func passRuns(seed uint64) (maxRun, passes, aba int) {
	cfg := sim.DefaultConfig()
	cfg.Mode = sim.Mode2v2
	cfg.PointsToWin = harnessPoints
	m := sim.NewMatch(cfg)
	var bots [sim.MaxPlayers]Bot
	for i := range bots {
		bots[i] = New(Hard, i)
		bots[i].Seed = seed*31 + uint64(i) + 1
	}
	run, streak := int8(-1), 0
	for tick := 0; tick < harnessMaxTicks; tick++ {
		s := m.State()
		if s.Phase == sim.PhaseGameOver {
			break
		}
		var in [sim.MaxPlayers]sim.PlayerInput
		for i := 0; i < int(s.NumPlayers); i++ {
			in[i] = bots[i].Think(&s, i)
		}
		for _, e := range m.Step(in) {
			switch e.Kind {
			case sim.EvKick:
				if e.Side == run {
					streak++
				} else {
					run, streak = e.Side, 1
				}
				if streak == 2 {
					passes++
				}
				if streak == 3 {
					aba++
				}
				maxRun = max(maxRun, streak)
			case sim.EvPoint:
				run, streak = -1, 0
			}
		}
	}
	return maxRun, passes, aba
}

// A bot that whiffed the rival's ball still plays its partner's set of that
// same ball: the miss lock is per touch, and passes do not bump ShotCount.
func TestBotWhoWhiffedStillTakesTheSet(t *testing.T) {
	m := setUpBall(1)
	bot := New(Hard, 0)
	bot.Level.MissChance = 0
	s := m.State()
	bot.missed, bot.missedShot, bot.missedTouch = true, s.Rally.ShotCount, touchKey{team: 1, player: 2, touches: 1}
	kicked := false
	for tick := 0; tick < 3*sim.TickRate && !kicked; tick++ {
		s := m.State()
		var in [sim.MaxPlayers]sim.PlayerInput
		in[0] = bot.Think(&s, 0)
		for _, e := range m.Step(in) {
			kicked = kicked || e.Kind == sim.EvKick && e.Player == 0
		}
	}
	if !kicked {
		t.Fatal("the bot that whiffed the rival's ball ignored its partner's set")
	}
}
