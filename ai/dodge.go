package ai

import (
	"math"

	"github.com/txny34/futenis/sim"
)

// dodgeMargin pads the body capsule (m) when keeping clear of the
// unbounced serve: the player keeps moving while the ball falls.
const dodgeMargin = 0.35

// dodgeServe keeps the receiver's body off the serve's path until it
// bounces (a body hit before the bounce is a touch: the point is lost,
// §21.6). If the believed ball would pass within the padded capsule of
// where the player stands or is heading, it steps sideways off the ball's line.
func (b *Bot) dodgeServe(s *sim.State, slot int, in sim.PlayerInput) sim.PlayerInput {
	me := &s.Players[slot]
	ball := b.belief
	const r = sim.PlayerRadius + sim.BallRadius + dodgeMargin
	for j := 1; j <= horizonTicks; j++ {
		var bounced bool
		ball, bounced = stepBall(ball, &s.Cfg)
		if bounced {
			return in
		}
		if ball.Pos.Y > me.Pos.Y+sim.PlayerHeight+sim.BallRadius {
			continue
		}
		pos := approach(me, b.tgt, j)
		near := math.Hypot(ball.Pos.X-pos.X, ball.Pos.Z-pos.Z) < r
		if near || math.Hypot(ball.Pos.X-me.Pos.X, ball.Pos.Z-me.Pos.Z) < r {
			return b.sidestep(me, ball, in)
		}
	}
	return in
}

// sidestep moves me perpendicular to ball's horizontal path, away from it.
func (b *Bot) sidestep(me *sim.Player, ball sim.Ball, in sim.PlayerInput) sim.PlayerInput {
	vx, vz := ball.Vel.X, ball.Vel.Z
	l := math.Hypot(vx, vz)
	if l < 1e-6 {
		vx, vz, l = 1, 0, 1
	}
	nx, nz := -vz/l, vx/l
	if (me.Pos.X-ball.Pos.X)*nx+(me.Pos.Z-ball.Pos.Z)*nz < 0 {
		nx, nz = -nx, -nz
	}
	in.MoveX, in.MoveZ = nx, nz
	in.Buttons &^= sim.BtnSprint | sim.BtnDash
	return in
}
