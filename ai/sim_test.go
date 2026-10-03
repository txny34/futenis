package ai

import (
	"testing"

	"github.com/txny34/futenis/sim"
)

const (
	harnessPoints   = 5
	harnessMatches  = 100
	harnessMaxTicks = 900 * sim.TickRate // 15 simulated minutes per match
	maxPointTicks   = 150 * sim.TickRate // a point over 150 s is treated as a stuck rally
)

type matchResult struct {
	winner        int8 // -1 if the tick cap was hit
	ticks         int
	longestPoint  int
	kicks, points int
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
	since := 0
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
			case sim.EvPoint:
				res.points++
				res.longestPoint = max(res.longestPoint, since)
				since = 0
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

// pairingStats aggregates harness results for one pairing.
type pairingStats struct {
	wins         [2]int
	kicks        int
	points       int
	longestPoint int
}

func (p *pairingStats) add(r matchResult, flipped bool) {
	w := int(r.winner)
	if flipped {
		w = 1 - w
	}
	p.wins[w]++
	p.kicks += r.kicks
	p.points += r.points
	p.longestPoint = max(p.longestPoint, r.longestPoint)
}

func (p *pairingStats) shotsPerPoint() float64 {
	if p.points == 0 {
		return 0
	}
	return float64(p.kicks) / float64(p.points)
}

// runPairing plays n 1v1 matches, alternating sides so neither difficulty
// always serves first; wins[0] counts a, wins[1] counts b.
func runPairing(t *testing.T, a, b Difficulty, n int) pairingStats {
	t.Helper()
	var st pairingStats
	for i := 0; i < n; i++ {
		flip := i%2 == 1
		diffs := [sim.MaxPlayers]Difficulty{a, b}
		if flip {
			diffs = [sim.MaxPlayers]Difficulty{b, a}
		}
		r := playMatch(sim.Mode1v1, diffs, uint64(i))
		if r.winner < 0 {
			t.Fatalf("%v vs %v match %d hit the tick cap (%d ticks, %d points)", a, b, i, r.ticks, r.points)
		}
		if r.longestPoint > maxPointTicks {
			t.Fatalf("%v vs %v match %d: a point lasted %d ticks (stuck rally)", a, b, i, r.longestPoint)
		}
		st.add(r, flip)
	}
	return st
}

func TestHarnessEveryPairingFinishes(t *testing.T) {
	if testing.Short() {
		t.Skip("long harness")
	}
	for a := Easy; a <= Pro; a++ {
		for b := a; b <= Pro; b++ {
			st := runPairing(t, a, b, harnessMatches)
			t.Logf("1v1 %-6v vs %-6v  wins %3d-%-3d  shots/point %.2f  longest point %.1fs",
				a, b, st.wins[0], st.wins[1], st.shotsPerPoint(), float64(st.longestPoint)/sim.TickRate)
			if a == Easy && b == Pro && st.wins[1] <= harnessMatches*8/10 {
				t.Errorf("Pro beat Easy only %d/%d", st.wins[1], harnessMatches)
			}
			if a == b && st.shotsPerPoint() < 2 {
				t.Errorf("%v mirror: %.2f shots/point, nobody returns", a, st.shotsPerPoint())
			}
		}
	}
}

func TestHarness2v2Finishes(t *testing.T) {
	if testing.Short() {
		t.Skip("long harness")
	}
	teams := [][sim.MaxPlayers]Difficulty{
		{Pro, Pro, Easy, Easy},
		{Normal, Hard, Hard, Normal},
		{Easy, Easy, Easy, Easy},
		{Pro, Pro, Pro, Pro},
	}
	for _, d := range teams {
		var st pairingStats
		for i := 0; i < 20; i++ {
			r := playMatch(sim.Mode2v2, d, uint64(i))
			if r.winner < 0 {
				t.Fatalf("2v2 %v match %d hit the tick cap (%d points)", d, i, r.points)
			}
			if r.longestPoint > maxPointTicks {
				t.Fatalf("2v2 %v match %d: a point lasted %d ticks", d, i, r.longestPoint)
			}
			st.add(r, false)
		}
		t.Logf("2v2 %v  wins %d-%d  shots/point %.2f  longest point %.1fs",
			d, st.wins[0], st.wins[1], st.shotsPerPoint(), float64(st.longestPoint)/sim.TickRate)
	}
}
