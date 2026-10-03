package sim

// PredictBall returns the ball's state after `ticks` fixed steps, pure (no
// State mutation, no events): same inputs always produce the same output.
// Shared by the strike timing window (wave1's improvesNextTick, §7.1) and
// the ai package's lookahead (§12).
//
// TODO(wave1): today this only mirrors stepBall's gravity+drag integration
// (no net/floor/wall bounce resolution), so Wave 0 callers compile and get
// a reasonable short-horizon prediction. strike.go/trajectory.go should
// replace the body with the same bounce-aware integration stepBall uses,
// kept event-free and non-mutating.
func PredictBall(b Ball, cfg *Config, ticks int) Ball {
	for i := 0; i < ticks; i++ {
		b.Vel.Y -= float64(cfg.Gravity * Dt)
		b.Vel = b.Vel.Scale(float64(1 - cfg.AirDrag*Dt))
		b.Pos = b.Pos.AddScaled(b.Vel, Dt)
	}
	return b
}
