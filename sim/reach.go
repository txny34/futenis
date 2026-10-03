package sim

import "math"

// reachShrinkSteps bounds how many times reachWithinCap pulls an
// unreachable target toward the net; reachShrink is the fraction of the
// distance past rivalMinTargetX that each step keeps.
const (
	reachShrinkSteps = 12
	reachShrink      = 0.75
)

// reachWithinCap is the aim assist for a shot the part cannot hit as fast
// as requested (docs/design/contact-v2.md §9, speed cap): instead of
// shrinking the velocity (which drops the ball into the net or short), it
// picks the flattest real trajectory to the same target that fits under
// maxSpeed and clears the net. If no flight time reaches the target under
// the cap, the target is pulled toward the net until one does, so the first
// contact is always the rival floor (never the glass, never the net). An
// own-side target keeps the old behaviour: a capped, unassisted shot.
func reachWithinCap(cfg *Config, from, target Vec3, maxSpeed float64) Vec3 {
	if sideOf(from.X) == sideOf(target.X) {
		return clampLen(launchVelocity(cfg, from, target, 1), maxSpeed)
	}
	t := target
	for i := 0; i < reachShrinkSteps; i++ {
		if v, ok := slowestLaunch(cfg, from, t, maxSpeed); ok {
			return v
		}
		sign := math.Copysign(1, t.X)
		t.X = sign * (rivalMinTargetX + float64((math.Abs(t.X)-rivalMinTargetX)*reachShrink))
	}
	return clampLen(launchVelocity(cfg, from, t, maxSolveTicks/4), maxSpeed)
}

// slowestLaunch scans flight times n (shortest first) for the first
// launchVelocity that fits under maxSpeed and clears the net. Longer flights
// are higher arcs, so the first hit is the flattest legal shot.
func slowestLaunch(cfg *Config, from, target Vec3, maxSpeed float64) (Vec3, bool) {
	for n := 1; n <= maxSolveTicks; n++ {
		v := launchVelocity(cfg, from, target, n)
		if v.Len() <= maxSpeed && netClearanceOK(cfg, from, v) {
			return v, true
		}
	}
	return Vec3{}, false
}
