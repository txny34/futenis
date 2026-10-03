package sim

import "math"

// Pure, event-free, allocation-free ball-trajectory lookahead (docs/design/
// contact-v2.md "Predicción (marcador de pique y anillo de timing)").
// PredictBall now mirrors stepBall's full net/floor/wall bounce resolution
// (physics.go) instead of only gravity+drag, so the strike timing window
// (strike.go's improvesNextTick) and the ai package's own lookahead share
// one bounce-aware implementation instead of ai/predict.go maintaining its
// own duplicate mirror of physics.go's bounce constants.
//
// PredictLanding/PredictContact build on PredictBall to answer the two
// questions the HUD needs for readability: where/when does the ball next
// hit the floor, and when/how would a given (stationary) player contact it
// if they stayed put. Neither mutates State or emits events.

// predictLandingHorizonTicks/predictContactHorizonTicks cap how far ahead
// PredictLanding/PredictContact look before giving up (ok=false), so a ball
// at rest or flying away forever can never loop unboundedly.
const (
	predictLandingHorizonTicks = TickRate * 3     // 3s
	predictContactHorizonTicks = TickRate * 3 / 2 // 1.5s (docs/design/contact-v2.md)
)

// PredictBall returns the ball's state after `ticks` fixed steps, pure (no
// State mutation, no events): same inputs always produce the same output.
// Shared by the strike timing window (strike.go's improvesNextTick, §7.1)
// and the ai package's lookahead (§12): bounce-aware (net/floor/wall,
// including the §10 spin model), exactly mirroring stepBall minus event
// emission.
func PredictBall(b Ball, cfg *Config, ticks int) Ball {
	for i := 0; i < ticks; i++ {
		b = predictStep(b, cfg)
	}
	return b
}

// predictStep advances b one fixed tick: gravity+drag integration, then the
// same net/floor/wall resolution order as stepBall (physics.go), without
// ever appending an Event or touching *State.
func predictStep(b Ball, cfg *Config) Ball {
	prevPos := b.Pos

	b.Vel.Y -= float64(cfg.Gravity * Dt)
	b.Vel = b.Vel.Scale(float64(1 - cfg.AirDrag*Dt))
	b.Pos = b.Pos.AddScaled(b.Vel, Dt)

	b = predictNet(b, prevPos)
	b = predictFloor(b, cfg)
	b = predictWalls(b, prevPos, cfg)
	return b
}

// predictNet mirrors resolveNet (physics.go): a crossing too low to clear
// the net is pushed back to the net post and loses most horizontal speed.
func predictNet(b Ball, prevPos Vec3) Ball {
	if sideOf(prevPos.X) == sideOf(b.Pos.X) {
		return b
	}
	t := (0 - prevPos.X) / (b.Pos.X - prevPos.X)
	yAtNet := prevPos.Y + float64((b.Pos.Y-prevPos.Y)*t)
	if yAtNet >= NetHeight+BallRadius {
		return b
	}
	sign := 1.0
	if prevPos.X < 0 {
		sign = -1.0
	}
	b.Pos.X = sign * BallRadius
	b.Vel.X = float64(-b.Vel.X * netBounceDamping)
	return b
}

// predictFloor mirrors resolveFloor (physics.go): bounces the ball with the
// §10 spin-adjusted restitution/friction, or lets it come to rest, without
// the Ball.Resting edge-triggering (no events to gate here -- every caller
// of PredictBall/PredictLanding only reads the returned Ball/tick, never
// Resting itself).
func predictFloor(b Ball, cfg *Config) Ball {
	if b.Pos.Y >= BallRadius {
		b.Resting = false
		return b
	}
	b.Pos.Y = BallRadius
	if b.Vel.Y >= 0 {
		return b
	}

	incoming := -b.Vel.Y
	if incoming < floorBounceMinSpeed {
		b.Vel.Y = 0
		b.Resting = true
		return b
	}

	b.Resting = false
	fric := clamp(float64(cfg.FloorFriction+cfg.SpinFloorFriction*b.Spin), floorSpinFrictionMin, floorSpinFrictionMax)
	rest := clamp(float64(cfg.FloorRestitution*(1-cfg.SpinFloorRest*b.Spin)), 0, floorSpinRestMax)
	b.Vel.Y = incoming * rest
	b.Vel.X *= fric
	b.Vel.Z *= fric
	b.Spin *= cfg.SpinDecay
	return b
}

// predictWalls mirrors resolveWalls/reflectWalls (physics.go): reflects the
// ball off the four court walls when it was inside the court last tick and
// is still low enough to hit a wall rather than fly over it.
func predictWalls(b Ball, prevPos Vec3, cfg *Config) Ball {
	prevInside := math.Abs(prevPos.X) <= CourtHalfLength && math.Abs(prevPos.Z) <= CourtHalfWidth
	if b.Pos.Y > WallHeight || !prevInside {
		return b
	}

	restW := clamp(float64(cfg.WallRestitution*(1+cfg.SpinWallRest*math.Min(b.Spin, 0))), 0, wallSpinRestMax)
	maxX := CourtHalfLength - BallRadius
	maxZ := CourtHalfWidth - BallRadius
	hit := false

	switch {
	case b.Pos.X > maxX:
		b.Pos.X = maxX
		b.Vel.X = float64(-b.Vel.X * restW)
		hit = true
	case b.Pos.X < -maxX:
		b.Pos.X = -maxX
		b.Vel.X = float64(-b.Vel.X * restW)
		hit = true
	}

	switch {
	case b.Pos.Z > maxZ:
		b.Pos.Z = maxZ
		b.Vel.Z = float64(-b.Vel.Z * restW)
		hit = true
	case b.Pos.Z < -maxZ:
		b.Pos.Z = -maxZ
		b.Vel.Z = float64(-b.Vel.Z * restW)
		hit = true
	}

	if hit {
		if b.Spin < 0 {
			b.Vel.Y -= float64(cfg.SpinWallDrop * (-b.Spin))
		}
		b.Spin *= cfg.SpinDecay
	}
	return b
}

// PredictLanding returns the ball's next floor contact (bounce OR coming to
// rest, same edge stepBall/resolveFloor would emit EvBounceFloor for): pos
// at that tick and how many ticks from now. ok is false if the ball never
// touches the floor within predictLandingHorizonTicks (e.g. it is already
// resting on it, or flying away with no downward crossing in the horizon).
func PredictLanding(s *State) (pos Vec3, ticks int, ok bool) {
	b := s.Ball
	cfg := &s.Cfg
	for t := 1; t <= predictLandingHorizonTicks; t++ {
		prevY := b.Pos.Y
		b = predictStep(b, cfg)
		if prevY > BallRadius && b.Pos.Y <= BallRadius {
			return b.Pos, t, true
		}
	}
	return Vec3{}, 0, false
}

// PredictContact returns the earliest tick, within predictContactHorizonTicks,
// at which the ball (predicted with PredictBall's bounce-aware physics)
// enters player's current contact envelope if they stay exactly where they
// are right now (docs/design/contact-v2.md "Predicción"): the body part
// that would make contact and the Quality that contact would resolve to
// (bestPart/qualityOf, the same tables strike.go uses). ok is false if no
// part ever reaches within the horizon (player is never bodyHit range of
// the predicted trajectory). player must be a valid, currently active
// State.Players index.
func PredictContact(s *State, player int) (ticks int, part BodyPart, quality Quality, ok bool) {
	if player < 0 || player >= MaxPlayers {
		return 0, PartNone, QualPerfect, false
	}
	p := &s.Players[player]
	cfg := &s.Cfg
	b := s.Ball

	for t := 1; t <= predictContactHorizonTicks; t++ {
		b = predictStep(b, cfg)
		hRel := b.Pos.Y - p.Pos.Y
		dH := math.Hypot(b.Pos.X-p.Pos.X, b.Pos.Z-p.Pos.Z)
		bp, u := bestPart(cfg, hRel, dH)
		if bp == PartNone {
			continue
		}
		return t, bp, qualityOf(cfg, u), true
	}
	return 0, PartNone, QualPerfect, false
}
