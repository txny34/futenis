package sim

import (
	"math"
	"testing"
)

// simulateFlight runs the same gravity+drag integration stepBall applies to
// the ball (no bounce resolution -- these tests never let the trajectory
// reach a wall/floor/net), so solver tests can check "does v0 actually land
// on target after N ticks of the REAL integrator" (docs/design/
// contact-v2.md §9) without depending on stepBall's side effects.
func simulateFlight(cfg *Config, b Ball, n int) Ball {
	for i := 0; i < n; i++ {
		b.Vel.Y -= float64(cfg.Gravity * Dt)
		b.Vel = b.Vel.Scale(kScale(cfg))
		b.Pos = b.Pos.AddScaled(b.Vel, Dt)
	}
	return b
}

func TestSolveLaunch_LandsExactlyOnTargetAtTickN(t *testing.T) {
	drags := []float64{0, 0.1, 0.3}
	targets := []Vec3{
		{X: 9, Y: BallRadius, Z: 0},
		{X: 3, Y: BallRadius, Z: 2},
		{X: -5, Y: BallRadius, Z: -4},
	}
	ticks := []int{5, 30, 120}

	for _, drag := range drags {
		cfg := DefaultConfig()
		cfg.AirDrag = drag
		from := Vec3{X: -2, Y: 1}

		for _, target := range targets {
			for _, n := range ticks {
				v := launchVelocity(&cfg, from, target, n)
				got := simulateFlight(&cfg, Ball{Pos: from, Vel: v}, n)

				const tol = 1e-6
				if math.Abs(got.Pos.X-target.X) > tol || math.Abs(got.Pos.Y-target.Y) > tol || math.Abs(got.Pos.Z-target.Z) > tol {
					t.Errorf("drag=%v n=%d target=%+v: landed at %+v, want within %v", drag, n, target, got.Pos, tol)
				}
			}
		}
	}
}

func TestSolveLaunch_DriveClearsNet(t *testing.T) {
	cfg := DefaultConfig()
	from := Vec3{X: -9, Y: 0.3} // far back wall
	target := Vec3{X: 5, Y: BallRadius}

	v := solveLaunch(&cfg, from, target, ShotDrive, 15, 1)

	if !netClearanceOK(&cfg, from, v) {
		t.Errorf("drive from %+v to %+v with v=%+v did not clear the net", from, target, v)
	}
}

func TestSolveLaunch_LobApexWithinTolerance(t *testing.T) {
	cfg := DefaultConfig()
	from := Vec3{X: -3, Y: 0.3}
	target := Vec3{X: 3, Y: BallRadius}
	charge := 0.5
	wantApex := cfg.LobApexMin + (cfg.LobApexMax-cfg.LobApexMin)*charge

	v := solveLaunch(&cfg, from, target, ShotLob, 10, charge)

	b := Ball{Pos: from, Vel: v}
	maxY := from.Y
	for i := 0; i < maxSolveTicks; i++ {
		b.Vel.Y -= float64(cfg.Gravity * Dt)
		b.Vel = b.Vel.Scale(kScale(&cfg))
		b.Pos = b.Pos.AddScaled(b.Vel, Dt)
		if b.Pos.Y > maxY {
			maxY = b.Pos.Y
		}
		if b.Pos.Y <= target.Y && i > 0 {
			break
		}
	}

	if math.Abs(maxY-wantApex) > 0.5 {
		t.Errorf("lob apex: got %v, want ~%v (+/- drag error)", maxY, wantApex)
	}
}

// simulateUntilFloor runs simulateFlight's integration one tick at a time
// and stops at the first tick the ball reaches the floor, returning its
// landing position -- unlike a fixed tick count, this matches "where does
// it actually land" regardless of how the clamp changed the flight time.
func simulateUntilFloor(cfg *Config, b Ball) Vec3 {
	for i := 0; i < maxSolveTicks; i++ {
		b.Vel.Y -= float64(cfg.Gravity * Dt)
		b.Vel = b.Vel.Scale(kScale(cfg))
		b.Pos = b.Pos.AddScaled(b.Vel, Dt)
		if b.Pos.Y <= BallRadius {
			return b.Pos
		}
	}
	return b.Pos
}

func TestSolveLaunch_OverSpeedClampedFallsShort(t *testing.T) {
	cfg := DefaultConfig()
	from := Vec3{X: -9.5, Y: 0.6}
	target := Vec3{X: 9.5, Y: BallRadius} // full court length

	v := solveLaunch(&cfg, from, target, ShotLob, 5, 0)
	rawLen := v.Len()
	cap := rawLen - 1 // force a cap strictly below what this shot actually needs
	v = clampLen(v, cap)
	landing := simulateUntilFloor(&cfg, Ball{Pos: from, Vel: v})

	if landing.X >= target.X {
		t.Errorf("clamped shot (raw %v > cap %v): got landing X=%v, want short of target X=%v", rawLen, cap, landing.X, target.X)
	}
}

func TestMomentum_RunningIntoShotIncreasesSpeed_Bounded(t *testing.T) {
	cfg := DefaultConfig()
	from := Vec3{X: 0}
	target := Vec3{X: 10}

	towards := momentumBonus(Vec3{X: 5}, from, target, &cfg)
	away := momentumBonus(Vec3{X: -5}, from, target, &cfg)
	still := momentumBonus(Vec3{}, from, target, &cfg)

	if towards <= still {
		t.Errorf("running into the shot: got bonus %v, want > stationary bonus %v", towards, still)
	}
	if away >= still {
		t.Errorf("running away from the shot: got bonus %v, want < stationary bonus %v", away, still)
	}
	huge := momentumBonus(Vec3{X: 1000}, from, target, &cfg)
	if huge != cfg.MomentumMax {
		t.Errorf("huge velocity: got bonus %v, want clamped to MomentumMax %v", huge, cfg.MomentumMax)
	}
}

func TestClampLen_LeavesShortVectorsUnchanged(t *testing.T) {
	v := Vec3{X: 3, Y: 4}
	got := clampLen(v, 10)
	if got != v {
		t.Errorf("got %+v, want unchanged %+v (already under maxLen)", got, v)
	}
}

func TestClampLen_ScalesLongVectorsDownToMaxLen(t *testing.T) {
	v := Vec3{X: 3, Y: 4} // len 5
	got := clampLen(v, 2.5)
	if math.Abs(got.Len()-2.5) > 1e-12 {
		t.Errorf("clampLen length: got %v, want 2.5", got.Len())
	}
}

func TestSeriesSum_MatchesDirectFormula(t *testing.T) {
	k := 0.95
	n := 10
	got := seriesSum(k, n)
	want := k * (1 - math.Pow(k, float64(n))) / (1 - k)
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("seriesSum: got %v, want %v", got, want)
	}
}
