package ai

import (
	"testing"

	"github.com/txny34/futenis/sim"
)

const (
	serveTrials    = 400
	serveTrialCap  = 8 * sim.TickRate
	serveSpotGrain = 0.25 // m: landing spots closer than this count as equal
)

// serveOutcome plays the first serve attempt of a fresh 1v1 padel match
// with bots seeded by trial and returns whether it faulted and, if it
// landed in the box, where.
func serveOutcome(d Difficulty, trial int) (fault bool, land sim.Vec3, ok bool) {
	cfg := sim.DefaultConfig()
	cfg.ScoringMode = sim.ScoringPadel
	m := sim.NewMatch(cfg)
	var bots [sim.MaxPlayers]Bot
	for i := range bots {
		bots[i] = New(d, i)
		bots[i].Seed = uint64(trial)*7 + uint64(i) + 1
	}
	for t := 0; t < serveTrialCap; t++ {
		s := m.State()
		var in [sim.MaxPlayers]sim.PlayerInput
		for i := 0; i < int(s.NumPlayers); i++ {
			in[i] = bots[i].Think(&s, i)
		}
		evs := m.Step(in)
		for _, e := range evs {
			if e.Kind == sim.EvServeFault {
				return true, sim.Vec3{}, true
			}
		}
		for _, e := range evs {
			if e.Kind == sim.EvBounceFloor && s.Rally.ServeBoxPending && e.Side != s.Server {
				return false, e.Pos, true
			}
		}
	}
	return false, sim.Vec3{}, false
}

// Serves vary (Level-driven, deterministic): easier bots miss the box more
// often (real faults), and the landing spots are not all the same.
func TestServeFaultRateAndVariety(t *testing.T) {
	bands := [difficultyCount][2]float64{
		Easy: {0.04, 0.13}, Normal: {0.015, 0.07}, Hard: {0.003, 0.04}, Pro: {0, 0.025},
	}
	prev := 1.0
	for d := Easy; d <= Pro; d++ {
		faults, n := 0, 0
		var spots []sim.Vec3
		for i := 0; i < serveTrials; i++ {
			f, land, ok := serveOutcome(d, i)
			if !ok {
				continue
			}
			n++
			if f {
				faults++
			} else {
				spots = append(spots, land)
			}
		}
		rate := float64(faults) / float64(max(n, 1))
		t.Logf("%v: %d/%d faults (%.1f%%), %d distinct landing spots", d, faults, n, 100*rate, distinctSpots(spots))
		if n < serveTrials*9/10 {
			t.Errorf("%v: only %d/%d serves were judged", d, n, serveTrials)
		}
		if rate < bands[d][0] || rate > bands[d][1] {
			t.Errorf("%v: fault rate %.3f outside [%.3f, %.3f]", d, rate, bands[d][0], bands[d][1])
		}
		if rate > prev {
			t.Errorf("%v: fault rate %.3f above the easier level's %.3f", d, rate, prev)
		}
		prev = rate
		if distinctSpots(spots) < 10 {
			t.Errorf("%v: serves land on only %d distinct spots", d, distinctSpots(spots))
		}
	}
}

func distinctSpots(spots []sim.Vec3) int {
	n := 0
	for i, a := range spots {
		dup := false
		for _, b := range spots[:i] {
			if a.Sub(b).Len() < serveSpotGrain {
				dup = true
				break
			}
		}
		if !dup {
			n++
		}
	}
	return n
}
