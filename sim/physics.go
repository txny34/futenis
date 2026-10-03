package sim

import "math"

// Tunable kick-shaping constants for the wave-0 v1-compat strike path (see
// performKick). Named so the aiming/lift math stays readable; these are
// removed once wave1's strike.go replaces this shim with the real §7-§9
// solver.
const (
	// kickAimMinForward is the minimum forward aim component enforced on
	// every compat kick, so a player can never aim the ball straight
	// sideways or backward.
	kickAimMinForward = 0.5
	// kickLoftDrive/kickLoftLob are the fixed vertical lift ratios used by a
	// compat Drive vs. Lob release (real charge-shaped lift is wave1).
	kickLoftDrive = 0.25
	kickLoftLob   = 0.6
	// kickPower is the fixed 0..1 power used by the compat shim (no real
	// charge measurement yet; see Player.ChargeTicks TODOs).
	kickPower = 1.0
	// netBounceDamping is the restitution applied to the X velocity when
	// the ball clips the net instead of crossing it cleanly.
	netBounceDamping = 0.3
	// floorBounceMinSpeed is the minimum downward speed (m/s) required for
	// a floor impact to bounce; slower impacts are treated as the ball
	// coming to rest (rolling) and emit no EvBounceFloor event.
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

// movePlayers advances every active player (s.NumPlayers) by one tick from
// their sanitized inputs, clamping each to their own team's half of the
// court and within the walls.
//
// TODO(wave1): same-team capsule separation (two partners should not
// overlap) is not implemented here; player.go adds it alongside jump/
// stamina/acceleration.
func movePlayers(s *State, inputs [MaxPlayers]PlayerInput) {
	for i := int8(0); i < s.NumPlayers; i++ {
		p := &s.Players[i]
		in := inputs[i]

		dir := Vec3{X: in.MoveX, Z: in.MoveZ}
		if l := dir.Len(); l > 1 {
			dir = dir.Scale(1 / l)
		}

		p.Vel = dir.Scale(s.Cfg.PlayerSpeed)
		p.Pos = p.Pos.AddScaled(p.Vel, Dt)
		p.Pos.Y = 0

		minX, maxX := sideXRange(p.Team)
		p.Pos.X = clamp(p.Pos.X, minX, maxX)
		zLimit := CourtHalfWidth - PlayerRadius
		p.Pos.Z = clamp(p.Pos.Z, -zLimit, zLimit)
	}
}

// canKick reports whether player index i is allowed to attempt a kick this
// tick: only the designated server (Rally.ServerPlayer) may kick during
// PhaseServe, anyone may kick during PhaseRally.
func canKick(s *State, i int8) bool {
	switch s.Phase {
	case PhaseServe:
		return i == s.Rally.ServerPlayer
	case PhaseRally:
		return true
	default:
		return false
	}
}

// applyKicks is the wave-0 compatibility shim for the v1 "Kick" path,
// temporarily driven by PlayerInput v2's Buttons instead of the removed
// edge-triggered Kick/AimX/AimZ/Loft/Power fields (docs/design/
// contact-v2.md §16, Wave 0 task: "map Buttons.Strike release -> old kick
// path"). It resolves at most one kick per tick (lowest player index wins
// a tie), appending an EvKick event when it connects.
//
// TODO(wave1): replaced wholesale by applyStrikes/bestPart/strike
// (strike.go, bodyparts.go) per §7.2 — real charge, body parts, deviation,
// the analytical solver and the simultaneous-kick tie-break (§7.3).
func applyKicks(s *State, inputs [MaxPlayers]PlayerInput, ev []Event) []Event {
	for i := int8(0); i < s.NumPlayers; i++ {
		p := &s.Players[i]
		in := inputs[i]
		shotBits := p.Held & (BtnStrike | BtnLob)
		wantsKick := released(p, in, BtnStrike) || released(p, in, BtnLob)
		p.Held = in.Buttons
		if !wantsKick || !canKick(s, i) {
			continue
		}
		reach := Vec3{X: p.Pos.X, Y: clamp(s.Ball.Pos.Y, 0, PlayerHeight), Z: p.Pos.Z}
		if s.Ball.Pos.Sub(reach).Len() > s.Cfg.KickReach {
			continue
		}
		ev = performKick(s, i, in, shotBits, ev)
		break
	}
	return ev
}

// performKick sets the ball's velocity from the kicking player's Target and
// the shot type latched in shotBits (BtnLob held => a lofted compat shot),
// promotes PhaseServe to PhaseRally, and emits EvKick with Side set to the
// kicker's TEAM (not their Players array index -- see Event.Player).
func performKick(s *State, i int8, in PlayerInput, shotBits uint8, ev []Event) []Event {
	p := &s.Players[i]
	fwd := forwardSign(p.Team)

	toTarget := Vec3{X: in.TargetX - s.Ball.Pos.X, Z: in.TargetZ - s.Ball.Pos.Z}
	if fwd*toTarget.X < kickAimMinForward {
		// Enforce a minimum forward component so a kick can never aim
		// straight sideways/backward, mirroring v1's kickAimMinForward.
		toTarget.X = fwd * kickAimMinForward
	}
	h := toTarget.Normalized()

	liftY := kickLoftDrive
	if shotBits&BtnLob != 0 {
		liftY = kickLoftLob
	}
	dir := h.Add(Vec3{Y: liftY}).Normalized()
	speed := s.Cfg.KickMinSpeed + float64((s.Cfg.KickMaxSpeed-s.Cfg.KickMinSpeed)*kickPower)

	s.Ball.Vel = dir.Scale(speed)
	s.Ball.Resting = false
	s.Ball.Out = false
	if s.Phase == PhaseServe {
		s.Phase = PhaseRally
	}
	return append(ev, Event{Kind: EvKick, Tick: s.Tick, Pos: s.Ball.Pos, Side: p.Team, Player: i})
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
		return append(ev, Event{Kind: EvBounceFloor, Tick: s.Tick, Pos: b.Pos, Side: sideOf(b.Pos.X), Player: -1})
	}

	b.Resting = false
	b.Vel.Y = incoming * s.Cfg.FloorRestitution
	b.Vel.X *= s.Cfg.FloorFriction
	b.Vel.Z *= s.Cfg.FloorFriction
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
// emitting one EvHitWall per wall actually hit this tick.
func reflectWalls(s *State, ev []Event) []Event {
	b := &s.Ball
	maxX := CourtHalfLength - BallRadius
	maxZ := CourtHalfWidth - BallRadius

	switch {
	case b.Pos.X > maxX:
		b.Pos.X = maxX
		b.Vel.X = float64(-b.Vel.X * s.Cfg.WallRestitution)
		ev = append(ev, Event{Kind: EvHitWall, Tick: s.Tick, Pos: b.Pos, Side: sideOf(b.Pos.X), Player: -1})
	case b.Pos.X < -maxX:
		b.Pos.X = -maxX
		b.Vel.X = float64(-b.Vel.X * s.Cfg.WallRestitution)
		ev = append(ev, Event{Kind: EvHitWall, Tick: s.Tick, Pos: b.Pos, Side: sideOf(b.Pos.X), Player: -1})
	}

	switch {
	case b.Pos.Z > maxZ:
		b.Pos.Z = maxZ
		b.Vel.Z = float64(-b.Vel.Z * s.Cfg.WallRestitution)
		ev = append(ev, Event{Kind: EvHitWall, Tick: s.Tick, Pos: b.Pos, Side: sideOf(b.Pos.X), Player: -1})
	case b.Pos.Z < -maxZ:
		b.Pos.Z = -maxZ
		b.Vel.Z = float64(-b.Vel.Z * s.Cfg.WallRestitution)
		ev = append(ev, Event{Kind: EvHitWall, Tick: s.Tick, Pos: b.Pos, Side: sideOf(b.Pos.X), Player: -1})
	}
	return ev
}
