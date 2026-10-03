package sim

import (
	"math"
	"testing"
)

// isFiniteVec3 reports whether every component of v is neither NaN nor Inf.
func isFiniteVec3(v Vec3) bool {
	return !math.IsNaN(v.X) && !math.IsInf(v.X, 0) &&
		!math.IsNaN(v.Y) && !math.IsInf(v.Y, 0) &&
		!math.IsNaN(v.Z) && !math.IsInf(v.Z, 0)
}

// TestApplyStrikes_FuzzNoNaN drives applyStrikes with a deterministic LCG
// script of randomized positions/charges/targets/spins across many seeds and
// asserts the ball state never goes non-finite, per the TDD mandate's "fuzz
// no NaN" requirement -- using the project's lcg helper instead of
// math/rand to stay inside sim's no-unseeded-randomness rule even in tests.
func TestApplyStrikes_FuzzNoNaN(t *testing.T) {
	for seed := uint64(1); seed <= 200; seed++ {
		g := newLCG(seed)
		s := newStrikeTestState()
		s.Tick = uint32(seed)

		s.Players[0].Pos = Vec3{X: g.floatRange(-10, 10), Z: g.floatRange(-5, 5)}
		s.Players[0].OnGround = g.bool(2)
		s.Players[0].Buffer = uint8(g.next() % 15)
		s.Players[0].BufCharge = g.floatRange(0, 1)
		s.Players[0].BufTarget = Vec3{
			X: g.floatRange(-10, 10),
			Y: BallRadius,
			Z: g.floatRange(-5, 5),
		}
		s.Players[0].BufSpin = int8(g.next()%3) - 1
		if g.bool(2) {
			s.Players[0].ShotType = ShotLob
		}
		s.Players[0].Vel = Vec3{X: g.floatRange(-10, 10), Z: g.floatRange(-10, 10)}

		s.Ball.Pos = Vec3{
			X: g.floatRange(-10, 10),
			Y: g.floatRange(0, 3),
			Z: g.floatRange(-5, 5),
		}

		ev := applyStrikes(&s, fullInputs(scriptedInput(g), scriptedInput(g)), nil)

		if !isFiniteVec3(s.Ball.Vel) || !isFiniteVec3(s.Ball.Pos) || math.IsNaN(s.Ball.Spin) || math.IsInf(s.Ball.Spin, 0) {
			t.Fatalf("seed %d: non-finite ball state after applyStrikes: Pos=%+v Vel=%+v Spin=%v (events=%+v)",
				seed, s.Ball.Pos, s.Ball.Vel, s.Ball.Spin, ev)
		}
		for _, e := range ev {
			if !isFiniteVec3(e.Vel) || math.IsNaN(e.Spin) || math.IsInf(e.Spin, 0) {
				t.Fatalf("seed %d: non-finite event payload: %+v", seed, e)
			}
		}
	}
}

// TestSolveLaunch_FuzzNoNaN exercises the analytic solver directly across
// randomized from/target/charge/speed combinations, including degenerate
// zero-distance and same-point cases.
func TestSolveLaunch_FuzzNoNaN(t *testing.T) {
	for seed := uint64(1); seed <= 200; seed++ {
		g := newLCG(seed)
		cfg := DefaultConfig()
		cfg.AirDrag = g.floatRange(0, 0.5)

		from := Vec3{X: g.floatRange(-10, 10), Y: g.floatRange(0, 3), Z: g.floatRange(-5, 5)}
		target := Vec3{X: g.floatRange(-10, 10), Y: BallRadius, Z: g.floatRange(-5, 5)}
		speed := g.floatRange(0, 30)
		charge := g.floatRange(0, 1)
		shot := ShotDrive
		if g.bool(2) {
			shot = ShotLob
		}

		v := solveLaunch(&cfg, from, target, shot, speed, charge)
		if !isFiniteVec3(v) {
			t.Fatalf("seed %d: non-finite solveLaunch result %+v (from=%+v target=%+v shot=%v speed=%v charge=%v drag=%v)",
				seed, v, from, target, shot, speed, charge, cfg.AirDrag)
		}
	}
}
