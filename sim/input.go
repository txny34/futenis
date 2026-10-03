package sim

import "math"

// sanitizeInput guards the deterministic simulation against invalid client
// input (docs/design/contact-v2.md §2). Unlike v1, clamping TargetX/TargetZ
// to the rival half depends on which side the player is on, so the function
// takes side explicitly; it also needs the live-tunable Target bounds, so it
// takes cfg (the doc's (in, side) signature omits cfg, but the clamp bounds
// -- TargetMinFromNet/TargetWallMargin -- are tunable Config fields, so a
// pure (in, side) signature can't reach them; see the wave-0 report for this
// deviation).
func sanitizeInput(in PlayerInput, side int8, cfg *Config) PlayerInput {
	return PlayerInput{
		MoveX:   clampAxis(in.MoveX),
		MoveZ:   clampAxis(in.MoveZ),
		TargetX: sanitizeTargetX(in.TargetX, side, cfg),
		TargetZ: sanitizeTargetZ(in.TargetZ, cfg),
		Buttons: in.Buttons & btnMask,
		Spin:    sanitizeSpin(in.Spin),
	}
}

// clampAxis sanitizes a -1..1 directional axis (MoveX/MoveZ).
func clampAxis(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	return clamp(v, -1, 1)
}

// sanitizeSpin clamps Spin to the documented discrete range {-1, 0, 1}.
func sanitizeSpin(v int8) int8 {
	if v < -1 {
		return -1
	}
	if v > 1 {
		return 1
	}
	return v
}

// sanitizeTargetX clamps TargetX to the rival half of the court for the
// given side: NaN/Inf collapses to the center of the rival half, otherwise
// its magnitude is clamped to [TargetMinFromNet, CourtHalfLength-
// TargetWallMargin] and given the sign that points at the rival half
// (forwardSign(side)).
func sanitizeTargetX(x float64, side int8, cfg *Config) float64 {
	fwd := forwardSign(side)
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return fwd * CourtHalfLength / 2
	}
	maxAbs := CourtHalfLength - cfg.TargetWallMargin
	absX := clamp(math.Abs(x), cfg.TargetMinFromNet, maxAbs)
	return fwd * absX
}

// sanitizeTargetZ clamps TargetZ to +/-(CourtHalfWidth-TargetWallMargin);
// NaN/Inf collapses to 0 (center).
func sanitizeTargetZ(z float64, cfg *Config) float64 {
	if math.IsNaN(z) || math.IsInf(z, 0) {
		return 0
	}
	limit := CourtHalfWidth - cfg.TargetWallMargin
	return clamp(z, -limit, limit)
}

// pressed reports whether bit transitioned from not-held to held this tick,
// comparing in.Buttons against p.Held (the previous tick's Buttons).
func pressed(p *Player, in PlayerInput, bit uint8) bool {
	return in.Buttons&bit != 0 && p.Held&bit == 0
}

// released reports whether bit transitioned from held to not-held this
// tick, comparing in.Buttons against p.Held (the previous tick's Buttons).
func released(p *Player, in PlayerInput, bit uint8) bool {
	return in.Buttons&bit == 0 && p.Held&bit != 0
}
