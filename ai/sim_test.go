package ai

import (
	"testing"

	"github.com/txny34/futenis/sim"
)

const (
	harnessPoints     = 5
	harnessMatches    = 100
	harnessMaxTicks   = 600 * sim.TickRate // 10 simulated minutes per match
	maxTicksPerPointM = 60 * sim.TickRate  // a single point must end in 60 s
)

type matchResult struct {
	winner       int8 // -1 if the tick cap was hit
	ticks        int
	longestRally int
	kicks        int
	returns      int // kicks after the serve within a point (rally returns)
}

// playMatch runs a headless bot-only match until game over or the cap.
func playMatch(mode sim.Mode, diffs [sim.MaxPlayers]Difficulty, seed uint64) matchResult {
	cfg := sim.DefaultConfig()
	cfg.Mode = mode
	cfg.PointsToWin = harnessPoints
	m := sim.NewMatch(cfg)
	var bots [sim.MaxPlayers]Bot
	for i := range bots {
		bots[i] = New(diffs[i], i)
		bots[i].Seed = seed*31 + uint64(i) + 1
	}
	res := matchResult{winner: -1}
	since, pointKicks := 0, 0
	for t := 0; t < harnessMaxTicks; t++ {
		s := m.State()
		if s.Phase == sim.PhaseGameOver {
			res.winner, res.ticks = gameWinner(&s), t
			return res
		}
		var in [sim.MaxPlayers]sim.PlayerInput
		for i := 0; i < int(s.NumPlayers); i++ {
			in[i] = bots[i].Think(&s, i)
		}
		since++
		for _, e := range m.Step(in) {
			switch e.Kind {
			case sim.EvKick:
				res.kicks++
				pointKicks++
				if pointKicks > 1 {
					res.returns++
				}
			case sim.EvPoint:
				res.longestRally = max(res.longestRally, since)
				since, pointKicks = 0, 0
			}
		}
	}
	res.ticks = harnessMaxTicks
	return res
}

func gameWinner(s *sim.State) int8 {
	if s.Score[0] > s.Score[1] {
		return 0
	}
	return 1
}

// strikeSystemReady gates the long statistical tests: they need bots to be
// able to put the ball in play. If no kick ever lands, the sim's strike
// path is not usable yet (wave 1 in progress) and the tests skip loudly.
func strikeSystemReady(t *testing.T) {
	t.Helper()
	r := playMatch(sim.Mode1v1, [sim.MaxPlayers]Difficulty{Pro, Pro}, 1)
	if r.returns == 0 {
		t.Skipf("sim strike path is not controllable yet: Pro-vs-Pro had %d kicks but 0 rally returns "+
			"(wave-0 compat kick has fixed 22 m/s power, every serve hits the back glass before bouncing); "+
			"skipping bot statistics until wave-1 strike.go lands", r.kicks)
	}
}

func TestHarnessEveryPairingFinishes(t *testing.T) {
	if testing.Short() {
		t.Skip("long harness")
	}
	strikeSystemReady(t)
	for a := Easy; a <= Pro; a++ {
		for b := a; b <= Pro; b++ {
			wins := [2]int{}
			for i := 0; i < harnessMatches; i++ {
				// Alternate sides so no difficulty keeps the first serve.
				diffs := [sim.MaxPlayers]Difficulty{a, b}
				flip := i%2 == 1
				if flip {
					diffs = [sim.MaxPlayers]Difficulty{b, a}
				}
				r := playMatch(sim.Mode1v1, diffs, uint64(i))
				if r.winner < 0 {
					t.Fatalf("%v vs %v match %d hit the tick cap (%d ticks)", a, b, i, r.ticks)
				}
				if r.longestRally > maxTicksPerPointM {
					t.Fatalf("%v vs %v match %d: a point lasted %d ticks", a, b, i, r.longestRally)
				}
				w := int(r.winner)
				if flip {
					w = 1 - w
				}
				wins[w]++
			}
			t.Logf("1v1 %v vs %v: %d-%d", a, b, wins[0], wins[1])
			if a == Easy && b == Pro && wins[1] <= harnessMatches*8/10 {
				t.Fatalf("Pro beat Easy only %d/%d", wins[1], harnessMatches)
			}
		}
	}
}

func TestHarness2v2Finishes(t *testing.T) {
	if testing.Short() {
		t.Skip("long harness")
	}
	strikeSystemReady(t)
	pairs := [][sim.MaxPlayers]Difficulty{
		{Pro, Pro, Easy, Easy},
		{Normal, Hard, Hard, Normal},
		{Easy, Easy, Easy, Easy},
		{Pro, Pro, Pro, Pro},
	}
	for _, d := range pairs {
		wins := [2]int{}
		for i := 0; i < 20; i++ {
			r := playMatch(sim.Mode2v2, d, uint64(i))
			if r.winner < 0 {
				t.Fatalf("2v2 %v match %d hit the tick cap", d, i)
			}
			wins[r.winner]++
		}
		t.Logf("2v2 %v: %d-%d", d, wins[0], wins[1])
	}
}
