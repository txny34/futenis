package sim

import "math"

// floorBounceMinSpeed is the minimum downward speed (m/s) required for a
// floor impact to bounce; slower impacts are treated as the ball coming to
// rest (rolling) and emit no EvBounceFloor event.
const floorBounceMinSpeed = 0.5

// netBounceDamping is the restitution applied to the X velocity when the
// ball clips the net instead of crossing it cleanly.
const netBounceDamping = 0.3

// floorSpinFrictionMin/Max and floorSpinRestMax bound the spin-adjusted
// floor friction/restitution so a pathological Config can't flip their sign
// or blow past a sane physical range (docs/design/contact-v2.md §10).
const (
	floorSpinFrictionMin = 0.4
	floorSpinFrictionMax = 1.05
	floorSpinRestMax     = 0.95
	wallSpinRestMax      = 1.0
)

// clamp returns v restricted to the inclusive range [lo, hi].
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// sideOf reports which half of the court an X coordinate belongs to: 0 for
// the negative-X side, 1 for the positive-X side (X == 0 counts as side 1).
func sideOf(x float64) int8 {
	if x < 0 {
		return 0
	}
	return 1
}

// sideXRange returns the inclusive X range a player on the given team is
// allowed to occupy, keeping them out of the net and the back wall.
func sideXRange(team int8) (min, max float64) {
	if team == 0 {
		return -CourtHalfLength + PlayerRadius, -PlayerRadius
	}
	return PlayerRadius, CourtHalfLength - PlayerRadius
}

// forwardSign returns the direction (+1 or -1 on X) a kick from the given
// team travels toward the opposite half of the court.
func forwardSign(team int8) float64 {
	if team == 0 {
		return 1
	}
	return -1
}

// canKick reports whether player index i is allowed to attempt a strike
// this tick: only the designated server (Rally.ServerPlayer) may strike
// during PhaseServe, anyone may strike during PhaseRally (in Training only
// while a machine ball is live).
func canKick(s *State, i int8) bool {
	switch s.Phase {
	case PhaseServe:
		return i == s.Rally.ServerPlayer
	case PhaseRally:
		// Training: no machine ball live (the gap between feeds) means nothing
		// to kick, so a strike released then is discarded (applyStrikes).
		return s.Cfg.Mode != ModePlayground || s.Feed.Live
	default:
		return false
	}
}

// stepBall integrates the ball one fixed tick under gravity and air drag,
// then resolves net, floor and wall collisions in that order, appending the
// corresponding events.
func stepBall(s *State, ev []Event) []Event {
	b := &s.Ball
	prevPos := b.Pos

	b.Vel.Y -= float64(s.Cfg.Gravity * Dt)
	b.Vel = b.Vel.Scale(float64(1 - s.Cfg.AirDrag*Dt))
	b.Pos = b.Pos.AddScaled(b.Vel, Dt)

	ev = resolveNet(s, prevPos, ev)
	ev = resolveFloor(s, ev)
	ev = resolveWalls(s, prevPos, ev)
	return ev
}

// resolveNet checks whether the ball crossed the net plane (X == 0) too low
// to clear it; if so it is pushed back to the net post and loses most of its
// horizontal speed, emitting EvHitNet.
func resolveNet(s *State, prevPos Vec3, ev []Event) []Event {
	b := &s.Ball
	if sideOf(prevPos.X) == sideOf(b.Pos.X) {
		return ev
	}

	t := (0 - prevPos.X) / (b.Pos.X - prevPos.X)
	yAtNet := prevPos.Y + float64((b.Pos.Y-prevPos.Y)*t)
	if yAtNet >= NetHeight+BallRadius {
		return ev
	}

	sign := 1.0
	if prevPos.X < 0 {
		sign = -1.0
	}
	b.Pos.X = sign * BallRadius
	b.Vel.X = float64(-b.Vel.X * netBounceDamping)
	return append(ev, Event{Kind: EvHitNet, Tick: s.Tick, Pos: b.Pos, Side: sideOf(prevPos.X), Player: -1})
}

// resolveFloor clamps the ball to the floor and either bounces it (scaling
// speed by a spin-adjusted restitution/friction, docs/design/contact-v2.md
// §10, and emitting EvBounceFloor) or lets it come to rest when the impact
// is too slow to count as a bounce. Coming to rest still emits
// EvBounceFloor once (edge-triggered via Ball.Resting) so the padel "second
// bounce" rule can still be judged even when the ball never bounces hard
// enough to leave the floor again; without this, a rally could get stuck
// forever once the ball settled instead of ending the point.
func resolveFloor(s *State, ev []Event) []Event {
	b := &s.Ball
	if b.Pos.Y >= BallRadius {
		b.Resting = false
		return ev
	}
	b.Pos.Y = BallRadius
	if b.Vel.Y >= 0 {
		return ev
	}

	incoming := -b.Vel.Y
	if incoming < floorBounceMinSpeed {
		b.Vel.Y = 0
		if b.Resting {
			return ev
		}
		b.Resting = true
		return append(ev, Event{Kind: EvBounceFloor, Tick: s.Tick, Pos: b.Pos, Side: sideOf(b.Pos.X), Player: -1})
	}

	b.Resting = false
	fric := clamp(float64(s.Cfg.FloorFriction+s.Cfg.SpinFloorFriction*b.Spin), floorSpinFrictionMin, floorSpinFrictionMax)
	rest := clamp(float64(s.Cfg.FloorRestitution*(1-s.Cfg.SpinFloorRest*b.Spin)), 0, floorSpinRestMax)
	b.Vel.Y = incoming * rest
	b.Vel.X *= fric
	b.Vel.Z *= fric
	b.Spin *= s.Cfg.SpinDecay
	return append(ev, Event{Kind: EvBounceFloor, Tick: s.Tick, Pos: b.Pos, Side: sideOf(b.Pos.X), Player: -1})
}

// resolveWalls reflects the ball off the four court walls when it was inside
// the court last tick and is still low enough to hit a wall rather than fly
// over it, then emits EvOut once (edge-triggered via Ball.Out) when the ball
// leaves the court bounds, instead of every tick it stays outside.
func resolveWalls(s *State, prevPos Vec3, ev []Event) []Event {
	b := &s.Ball
	prevInside := math.Abs(prevPos.X) <= CourtHalfLength && math.Abs(prevPos.Z) <= CourtHalfWidth
	if b.Pos.Y <= WallHeight && prevInside {
		ev = reflectWalls(s, ev)
	}

	outside := math.Abs(b.Pos.X) > CourtHalfLength || math.Abs(b.Pos.Z) > CourtHalfWidth
	if !outside {
		b.Out = false
		return ev
	}
	if b.Out {
		return ev
	}
	b.Out = true
	return append(ev, Event{Kind: EvOut, Tick: s.Tick, Pos: b.Pos, Side: sideOf(b.Pos.X), Player: -1})
}

// reflectWalls performs the actual X/Z wall reflections for resolveWalls,
// emitting one EvHitWall per wall actually hit this tick, then applies the
// spin model's wall effect once for the whole tick (docs/design/
// contact-v2.md §10): backspin eats restitution and pushes the ball down
// ("la bandeja muere en el vidrio"); spin decays per bounce regardless of
// sign.
func reflectWalls(s *State, ev []Event) []Event {
	b := &s.Ball
	cfg := &s.Cfg
	restW := clamp(float64(cfg.WallRestitution*(1+cfg.SpinWallRest*math.Min(b.Spin, 0))), 0, wallSpinRestMax)
	maxX := CourtHalfLength - BallRadius
	maxZ := CourtHalfWidth - BallRadius
	hit := false

	switch {
	case b.Pos.X > maxX:
		b.Pos.X = maxX
		b.Vel.X = float64(-b.Vel.X * restW)
		ev = append(ev, Event{Kind: EvHitWall, Tick: s.Tick, Pos: b.Pos, Side: sideOf(b.Pos.X), Player: -1})
		hit = true
	case b.Pos.X < -maxX:
		b.Pos.X = -maxX
		b.Vel.X = float64(-b.Vel.X * restW)
		ev = append(ev, Event{Kind: EvHitWall, Tick: s.Tick, Pos: b.Pos, Side: sideOf(b.Pos.X), Player: -1})
		hit = true
	}

	switch {
	case b.Pos.Z > maxZ:
		b.Pos.Z = maxZ
		b.Vel.Z = float64(-b.Vel.Z * restW)
		ev = append(ev, Event{Kind: EvHitWall, Tick: s.Tick, Pos: b.Pos, Side: sideOf(b.Pos.X), Player: -1})
		hit = true
	case b.Pos.Z < -maxZ:
		b.Pos.Z = -maxZ
		b.Vel.Z = float64(-b.Vel.Z * restW)
		ev = append(ev, Event{Kind: EvHitWall, Tick: s.Tick, Pos: b.Pos, Side: sideOf(b.Pos.X), Player: -1})
		hit = true
	}

	if hit {
		if b.Spin < 0 {
			b.Vel.Y -= float64(cfg.SpinWallDrop * (-b.Spin))
		}
		b.Spin *= cfg.SpinDecay
	}
	return ev
}
