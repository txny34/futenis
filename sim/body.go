package sim

import "math"

// body.go: ball-vs-player capsule collision (docs/design/contact-v2.md
// §7.4). Wave1 ownership: emits EvBodyHit, which counts as a touch for the
// hitter's team -- rules.go (not owned here) is responsible for judging
// that.

// applyBodyCollisions resolves ball-vs-player capsule collisions for every
// active player, after the ball has moved this tick. At most the ball's
// velocity/position are mutated per contact; multiple players could in
// principle all be in range on the same tick (e.g. 2v2 partners standing
// together), so every active player is checked in a fixed index order,
// deterministically.
func applyBodyCollisions(s *State, ev []Event) []Event {
	for i := int8(0); i < s.NumPlayers; i++ {
		ev = resolveBodyHit(s, i, ev)
	}
	return ev
}

// resolveBodyHit checks player i's capsule against the ball and, on a real
// incoming contact, reflects the ball off it with BodyRestitution,
// depenetrates, decays spin and emits EvBodyHit. The player who struck the
// ball within the last graceTicks is immune to colliding with their own
// body (so a shot can leave the kicker's own hitbox without immediately
// bouncing off it again). A player with a strike still buffered is skipped
// too: the strike is waiting for its best contact tick and owns the ball,
// so the capsule must not touch it first (that touch plus the strike one
// tick later would be judged a double touch).
func resolveBodyHit(s *State, i int8, ev []Event) []Event {
	p := &s.Players[i]
	cfg := &s.Cfg
	grace := uint32(clampTickCount(cfg.graceTicks, math.MaxInt32))
	if p.Buffer > 0 || (p.StruckAt != 0 && s.Tick-p.StruckAt < grace) {
		return ev
	}

	c := closestPointOnCapsule(p, s.Ball.Pos)
	d := s.Ball.Pos.Sub(c)
	dist := d.Len()
	minDist := PlayerRadius + BallRadius
	if dist >= minDist {
		return ev
	}

	n := bodyNormal(d, dist, p.Team)
	rel := s.Ball.Vel.Sub(p.Vel)
	vn := rel.Dot(n)
	if vn >= 0 {
		// Ball and player already separating along the contact normal: not
		// an incoming hit (e.g. still overlapping from a previous tick's
		// depenetration), nothing to resolve.
		return ev
	}

	s.Ball.Vel = s.Ball.Vel.Sub(n.Scale(float64(vn * (1 + cfg.BodyRestitution))))
	s.Ball.Pos = c.Add(n.Scale(minDist))
	s.Ball.Spin *= cfg.SpinDecay
	return append(ev, Event{Kind: EvBodyHit, Tick: s.Tick, Pos: s.Ball.Pos, Side: p.Team, Player: i})
}

// closestPointOnCapsule returns the point on player p's vertical capsule
// segment (Pos.XZ, Y clamped to [Pos.Y+PlayerRadius, Pos.Y+PlayerHeight-
// PlayerRadius]) closest to ballPos.
func closestPointOnCapsule(p *Player, ballPos Vec3) Vec3 {
	yMin := p.Pos.Y + PlayerRadius
	yMax := p.Pos.Y + PlayerHeight - PlayerRadius
	y := clamp(ballPos.Y, yMin, yMax)
	return Vec3{X: p.Pos.X, Y: y, Z: p.Pos.Z}
}

// bodyNormal is the outward contact normal from the capsule toward the
// ball. When the ball's center sits exactly on the capsule's segment
// (dist == 0, a degenerate case that would otherwise divide by zero), it
// falls back to the player's forward direction (docs/design/contact-v2.md
// §7.4: "dist==0 -> n = forward(side)").
func bodyNormal(d Vec3, dist float64, team int8) Vec3 {
	if dist == 0 {
		return Vec3{X: forwardSign(team)}
	}
	return d.Scale(1 / dist)
}
