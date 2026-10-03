package sim

import "math"

// Tunable kick-shaping constants. Named so the aiming/lift math in
// performKick stays readable and the magic numbers have a single home.
const (
	// kickAimMinForward is the minimum forward aim component (fraction of
	// KickMinSpeed..KickMaxSpeed direction) enforced on every kick, so a
	// player can never aim the ball straight sideways or backward.
	kickAimMinForward = 0.5
	// kickLoftBase is the baseline vertical lift added to every kick (unit
	// direction units), so a Loft of 0 still clears the net at short range.
	kickLoftBase = 0.15
	// kickLoftScale is how much additional vertical lift Loft (0..1) adds.
	kickLoftScale = 1.5
	// netBounceDamping is the restitution applied to the X velocity when the
	// ball clips the net instead of crossing it cleanly.
	netBounceDamping = 0.3
	// floorBounceMinSpeed is the minimum downward speed (m/s) required for a
	// floor impact to bounce; slower impacts are treated as the ball coming
	// to rest (rolling) and emit no EvBounceFloor event.
	floorBounceMinSpeed = 0.5
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

// sideXRange returns the inclusive X range a player on the given side is
// allowed to occupy, keeping them out of the net and the back wall.
func sideXRange(side int8) (min, max float64) {
	if side == 0 {
		return -CourtHalfLength + PlayerRadius, -PlayerRadius
	}
	return PlayerRadius, CourtHalfLength - PlayerRadius
}

// forwardSign returns the direction (+1 or -1 on X) a kick from the given
// side travels toward the opposite half of the court.
func forwardSign(side int8) float64 {
	if side == 0 {
		return 1
	}
	return -1
}

// movePlayers advances both players by one tick from their sanitized inputs,
// clamping each player to their own half of the court and within the walls.
func movePlayers(s *State, inputs [2]PlayerInput) {
	for i := range s.Players {
		p := &s.Players[i]
		in := inputs[i]

		dir := Vec3{X: in.MoveX, Z: in.MoveZ}
		if l := dir.Len(); l > 1 {
			dir = dir.Scale(1 / l)
		}

		p.Vel = dir.Scale(s.Cfg.PlayerSpeed)
		p.Pos = p.Pos.AddScaled(p.Vel, Dt)
		p.Pos.Y = 0

		minX, maxX := sideXRange(p.Side)
		p.Pos.X = clamp(p.Pos.X, minX, maxX)
		zLimit := CourtHalfWidth - PlayerRadius
		p.Pos.Z = clamp(p.Pos.Z, -zLimit, zLimit)
	}
}

// canKick reports whether side i is allowed to attempt a kick this tick: only
// the server may kick during PhaseServe, anyone may kick during PhaseRally.
func canKick(s *State, i int8) bool {
	switch s.Phase {
	case PhaseServe:
		return i == s.Server
	case PhaseRally:
		return true
	default:
		return false
	}
}

// applyKicks resolves at most one kick per tick (side 0 takes priority over
// side 1 when both attempt one), appending an EvKick event when it connects.
// Enforcing "max 1 touch per side" across a rally is rules.go's job, not
// this function's.
func applyKicks(s *State, inputs [2]PlayerInput, ev []Event) []Event {
	for i := int8(0); i < int8(len(s.Players)); i++ {
		in := inputs[i]
		if !in.Kick || !canKick(s, i) {
			continue
		}
		p := &s.Players[i]
		reach := Vec3{X: p.Pos.X, Y: clamp(s.Ball.Pos.Y, 0, PlayerHeight), Z: p.Pos.Z}
		if s.Ball.Pos.Sub(reach).Len() > s.Cfg.KickReach {
			continue
		}
		// TODO(contact-v2): simultaneous kicks always let side 0 win the
		// tie-break below (loop order, i == 0 first). Revisit once contact
		// v2 models real reach/timing instead of a single per-tick check.
		ev = performKick(s, i, in, ev)
		break
	}
	return ev
}

// performKick sets the ball's velocity from the kicking player's aim, loft
// and power, promotes PhaseServe to PhaseRally, and emits EvKick.
func performKick(s *State, i int8, in PlayerInput, ev []Event) []Event {
	fwd := forwardSign(i)
	ax := math.Max(math.Abs(in.AimX), kickAimMinForward)
	h := Vec3{X: fwd * ax, Z: in.AimZ}.Normalized()
	liftY := kickLoftBase + float64(in.Loft*kickLoftScale)
	dir := h.Add(Vec3{Y: liftY}).Normalized()
	speed := s.Cfg.KickMinSpeed + float64((s.Cfg.KickMaxSpeed-s.Cfg.KickMinSpeed)*in.Power)

	s.Ball.Vel = dir.Scale(speed)
	s.Ball.Resting = false
	s.Ball.Out = false
	if s.Phase == PhaseServe {
		s.Phase = PhaseRally
	}
	return append(ev, Event{Kind: EvKick, Tick: s.Tick, Pos: s.Ball.Pos, Side: i})
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
	return append(ev, Event{Kind: EvHitNet, Tick: s.Tick, Pos: b.Pos, Side: sideOf(prevPos.X)})
}

// resolveFloor clamps the ball to the floor and either bounces it (scaling
// speed by FloorRestitution/FloorFriction and emitting EvBounceFloor) or lets
// it come to rest when the impact is too slow to count as a bounce. Coming
// to rest still emits EvBounceFloor once (edge-triggered via Ball.Resting)
// so the padel "second bounce" rule can still be judged even when the ball
// never bounces hard enough to leave the floor again; without this, a rally
// could get stuck forever once the ball settled instead of ending the point.
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
		return append(ev, Event{Kind: EvBounceFloor, Tick: s.Tick, Pos: b.Pos, Side: sideOf(b.Pos.X)})
	}

	b.Resting = false
	b.Vel.Y = incoming * s.Cfg.FloorRestitution
	b.Vel.X *= s.Cfg.FloorFriction
	b.Vel.Z *= s.Cfg.FloorFriction
	return append(ev, Event{Kind: EvBounceFloor, Tick: s.Tick, Pos: b.Pos, Side: sideOf(b.Pos.X)})
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
	return append(ev, Event{Kind: EvOut, Tick: s.Tick, Pos: b.Pos, Side: sideOf(b.Pos.X)})
}

// reflectWalls performs the actual X/Z wall reflections for resolveWalls,
// emitting one EvHitWall per wall actually hit this tick.
func reflectWalls(s *State, ev []Event) []Event {
	b := &s.Ball
	maxX := CourtHalfLength - BallRadius
	maxZ := CourtHalfWidth - BallRadius

	switch {
	case b.Pos.X > maxX:
		b.Pos.X = maxX
		b.Vel.X = float64(-b.Vel.X * s.Cfg.WallRestitution)
		ev = append(ev, Event{Kind: EvHitWall, Tick: s.Tick, Pos: b.Pos, Side: sideOf(b.Pos.X)})
	case b.Pos.X < -maxX:
		b.Pos.X = -maxX
		b.Vel.X = float64(-b.Vel.X * s.Cfg.WallRestitution)
		ev = append(ev, Event{Kind: EvHitWall, Tick: s.Tick, Pos: b.Pos, Side: sideOf(b.Pos.X)})
	}

	switch {
	case b.Pos.Z > maxZ:
		b.Pos.Z = maxZ
		b.Vel.Z = float64(-b.Vel.Z * s.Cfg.WallRestitution)
		ev = append(ev, Event{Kind: EvHitWall, Tick: s.Tick, Pos: b.Pos, Side: sideOf(b.Pos.X)})
	case b.Pos.Z < -maxZ:
		b.Pos.Z = -maxZ
		b.Vel.Z = float64(-b.Vel.Z * s.Cfg.WallRestitution)
		ev = append(ev, Event{Kind: EvHitWall, Tick: s.Tick, Pos: b.Pos, Side: sideOf(b.Pos.X)})
	}
	return ev
}
