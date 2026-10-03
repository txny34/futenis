package sim

import "math"

// maxSolveTicks bounds every loop in this file that walks tick-by-tick
// (series sums, net-clearance search): at TickRate=120 a 5-second flight is
// already an absurdly long shot, so this is a generous, deterministic upper
// bound that keeps every loop here O(1) in practice without ever touching
// math.Pow (docs/design/contact-v2.md §9: "N <= 600 a 120 Hz").
const maxSolveTicks = 600

// netClearanceIterations is how many times driveTicks is allowed to grow N
// and re-solve looking for a trajectory that clears the net with the
// configured margin (docs/design/contact-v2.md §9: "máx 12 iteraciones,
// acotado").
const netClearanceIterations = 12

// clampLen scales v down to maxLen if it is longer, leaving it untouched
// (including the zero vector) otherwise.
func clampLen(v Vec3, maxLen float64) Vec3 {
	l := v.Len()
	if l <= maxLen || l == 0 {
		return v
	}
	return v.Scale(maxLen / l)
}

// kScale is 1-AirDrag*Dt: the per-tick velocity scale factor stepBall
// actually applies. Solving against this exact recurrence (instead of the
// continuous exponential-drag formula) is what makes solveLaunch land
// exactly on target at tick N (docs/design/contact-v2.md §9).
func kScale(cfg *Config) float64 {
	return float64(1 - cfg.AirDrag*Dt)
}

// seriesSum returns S = sum_{i=1}^{n} k^i, computed with the same
// repeated-multiplication order stepBall's Vel.Scale(k) chain uses (no
// math.Pow), so S is consistent tick-for-tick with the real integrator.
func seriesSum(k float64, n int) float64 {
	s := 0.0
	term := 1.0
	for i := 0; i < n; i++ {
		term = float64(term * k)
		s = float64(s + term)
	}
	return s
}

// verticalCorrection is the g*Dt*k/(1-k)*(N-S) term of the §9 vertical
// solve. At k==1 (AirDrag==0) that expression is a 0/0 limit; the closed
// form for undamped uniform acceleration is g*Dt*N*(N+1)/2 (derived from
// sum_{n=1}^{N} n = N*(N+1)/2), used directly instead of dividing by zero.
func verticalCorrection(g, dt, k float64, n int, s float64) float64 {
	if k == 1 {
		return float64(g*dt) * (float64(n*(n+1)) / 2)
	}
	return float64(g*dt) * (k / (1 - k)) * (float64(n) - s)
}

// launchVelocity returns the ball velocity at `from` that reaches `target`
// in exactly n ticks of stepBall's actual discrete gravity+drag integrator
// (docs/design/contact-v2.md §9), solving the recurrence directly instead
// of a continuous approximation. n must be >= 1.
func launchVelocity(cfg *Config, from, target Vec3, n int) Vec3 {
	k := kScale(cfg)
	s := seriesSum(k, n)
	if s == 0 {
		s = 1e-9
	}
	dtS := float64(Dt * s)

	vx := (target.X - from.X) / dtS
	vz := (target.Z - from.Z) / dtS

	dy := target.Y - from.Y
	corr := verticalCorrection(cfg.Gravity, Dt, k, n, s)
	vy := (dy/Dt + corr) / s

	return Vec3{X: vx, Y: vy, Z: vz}
}

// netClearanceOK simulates (pure, no events) the trajectory launched with v
// from `from` and reports whether it clears the net plane (X==0) by at
// least cfg.DriveNetClearance above NetHeight+BallRadius, the moment it
// crosses sides. A shot that never crosses sides (already won/lost before
// the net) trivially clears.
func netClearanceOK(cfg *Config, from, v Vec3) bool {
	b := Ball{Pos: from, Vel: v}
	prevSide := sideOf(b.Pos.X)
	for i := 0; i < maxSolveTicks; i++ {
		b.Vel.Y -= float64(cfg.Gravity * Dt)
		b.Vel = b.Vel.Scale(kScale(cfg))
		b.Pos = b.Pos.AddScaled(b.Vel, Dt)
		if side := sideOf(b.Pos.X); side != prevSide {
			return b.Pos.Y >= NetHeight+BallRadius+cfg.DriveNetClearance
		}
	}
	return true
}

// driveTicks picks N for a Drive shot (docs/design/contact-v2.md §9):
// start from the no-drag estimate d/(vh*Dt), refine it against the actual
// k-weighted average speed with two fixed-point iterations, then grow N
// (re-solving each time) until the resulting trajectory clears the net by
// DriveNetClearance, or netClearanceIterations is exhausted.
func driveTicks(cfg *Config, from, target Vec3, d, vh float64) int {
	if vh <= 0 {
		vh = 1e-6
	}
	k := kScale(cfg)
	n := int(math.Ceil(d / (vh * Dt)))
	if n < 1 {
		n = 1
	}
	for iter := 0; iter < 2; iter++ {
		s := seriesSum(k, n)
		kbar := s / float64(n)
		if kbar <= 0 {
			kbar = 1e-6
		}
		n = int(math.Ceil(d / (vh * Dt * kbar)))
		if n < 1 {
			n = 1
		}
	}
	return ensureNetClearance(cfg, from, target, n)
}

// ensureNetClearance grows n until launchVelocity(n) clears the net with
// the configured margin, or gives up after netClearanceIterations (a
// deliberately short/over-powered shot is allowed to clip the net — that is
// the "red = castigo legítimo" mechanic, not a bug).
func ensureNetClearance(cfg *Config, from, target Vec3, n int) int {
	if sideOf(from.X) == sideOf(target.X) {
		return n
	}
	for iter := 0; iter < netClearanceIterations; iter++ {
		v := launchVelocity(cfg, from, target, n)
		if netClearanceOK(cfg, from, v) {
			return n
		}
		n++
	}
	return n
}

// lobTicks picks N for a Lob shot from the desired apex height (linearly
// interpolated between LobApexMin/Max by charge), using the no-drag
// time-of-flight approximation from docs/design/contact-v2.md §9: the real
// drag-affected apex ends up slightly lower, which is accepted/tunable.
func lobTicks(cfg *Config, y0, yT, charge float64) int {
	apex := cfg.LobApexMin + float64((cfg.LobApexMax-cfg.LobApexMin)*charge)
	if apex < y0 {
		apex = y0
	}
	if apex < yT {
		apex = yT
	}
	g := cfg.Gravity
	t1 := math.Sqrt(2 * (apex - y0) / g)
	t2 := math.Sqrt(2 * (apex - yT) / g)
	n := int(math.Round((t1 + t2) / Dt))
	if n < 1 {
		n = 1
	}
	return n
}

// solveLaunch is the §9 entry point: pick N by shot type (drive clears the
// net, lob targets an apex), then solve the exact launch velocity for that
// N against the real discrete integrator.
func solveLaunch(cfg *Config, from, target Vec3, shot ShotType, speed, charge float64) Vec3 {
	d := math.Hypot(target.X-from.X, target.Z-from.Z)
	var n int
	if shot == ShotLob {
		n = lobTicks(cfg, from.Y, target.Y, charge)
	} else {
		n = driveTicks(cfg, from, target, d, speed)
	}
	return launchVelocity(cfg, from, target, n)
}

// momentumBonus is the §9 momentum contribution: the player's own velocity,
// projected onto the shot direction, scaled by MomentumFactor and clamped
// to +/-MomentumMax. Running into the shot speeds it up; running away from
// it slows it down.
func momentumBonus(vel, from, target Vec3, cfg *Config) float64 {
	dir := Vec3{X: target.X - from.X, Z: target.Z - from.Z}.Normalized()
	bonus := float64(cfg.MomentumFactor * vel.Dot(dir))
	return clamp(bonus, -cfg.MomentumMax, cfg.MomentumMax)
}
