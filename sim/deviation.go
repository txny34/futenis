package sim

// deviateTarget applies the §8 deterministic target deviation: a radius R
// that grows with u² (so a forced contact is punished much harder than a
// merely-good one), split into a longitudinal component (along the
// ball->target axis, weight 1) and a lateral one (perpendicular, weight
// 0.6, "más error en largo que en lateral"). The result is re-clamped only
// on Z, per the doc: an X deviation is allowed to leave the shot short/long
// (a real miss), only the sideline is a hard wall.
func deviateTarget(cfg *Config, target, ballXZ Vec3, ps *PartSpec, u float64, h uint64) Vec3 {
	r := ps.DevBase + float64(u*u)*(ps.DevMax-ps.DevBase)
	a, b := unitPair(h)

	dir := Vec3{X: target.X - ballXZ.X, Z: target.Z - ballXZ.Z}.Normalized()
	perp := Vec3{X: -dir.Z, Z: dir.X}

	deviated := Vec3{
		X: target.X + float64(dir.X*(a*r)) + float64(perp.X*(b*r*0.6)),
		Y: target.Y,
		Z: target.Z + float64(dir.Z*(a*r)) + float64(perp.Z*(b*r*0.6)),
	}

	limit := CourtHalfWidth - cfg.TargetWallMargin
	deviated.Z = clamp(deviated.Z, -limit, limit)
	return deviated
}

// shotSpeed is the §8 power deviation: a base speed linearly interpolated
// across the part's [SpeedMin, SpeedMax] range by charge, perturbed by up to
// +/-DevPowerMax scaled by the same error u (a perfect contact, u≈0, barely
// deviates power at all).
func shotSpeed(ps *PartSpec, charge, u float64, h uint64, cfg *Config) float64 {
	base := ps.SpeedMin + float64((ps.SpeedMax-ps.SpeedMin)*charge)
	a, _ := unitPair(h)
	c := a * u
	return float64(base * (1 + c*cfg.DevPowerMax))
}
