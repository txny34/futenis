package ai

import (
	"math"

	"github.com/txny34/futenis/sim"
)

// releaseScan bounds the release lookahead (ticks, fixed array: no allocs).
const releaseScan = 48

// contactProfile is the believed ball's contact error against a player
// heading for its target, tick by tick from now (index 0 =
// the ball this tick, which the sim strikes before moving it).
type contactProfile struct {
	n     int
	u     [releaseScan]float64 // best part error, 2 = out of reach
	body  [releaseScan]bool    // the ball would touch the body capsule
	early [releaseScan]bool    // reachable before the required bounces
}

// profile fills c for n ticks (n <= releaseScan).
func (c *contactProfile) profile(ball sim.Ball, p *sim.Player, tgt sim.Vec3, cfg *sim.Config, n, minBounces int) {
	c.n = min(n, releaseScan)
	bounces := 0
	for j := 0; j < c.n; j++ {
		if j > 0 {
			var b bool
			ball, b = stepBall(ball, cfg)
			if b {
				bounces++
			}
		}
		pos := approach(p, tgt, j)
		dH := math.Hypot(ball.Pos.X-pos.X, ball.Pos.Z-pos.Z)
		hRel := ball.Pos.Y - pos.Y
		_, c.u[j] = sim.BestPart(cfg, hRel, dH, false)
		c.early[j] = bounces < minBounces && c.u[j] < 2
		c.body[j] = dH < sim.PlayerRadius+sim.BallRadius && hRel < sim.PlayerHeight
	}
}

// approach predicts p's position j ticks ahead while it heads for tgt
// (XZ): it keeps its current speed and stops on tgt instead of running
// past it, as moveTo's arrival easing does.
func approach(p *sim.Player, tgt sim.Vec3, j int) sim.Vec3 {
	dx, dz := tgt.X-p.Pos.X, tgt.Z-p.Pos.Z
	d := math.Hypot(dx, dz)
	step := math.Hypot(p.Vel.X, p.Vel.Z) * float64(j) * sim.Dt
	if d < 1e-6 || step >= d {
		return sim.Vec3{X: tgt.X, Y: p.Pos.Y, Z: tgt.Z}
	}
	f := step / d
	return sim.Vec3{X: p.Pos.X + dx*f, Y: p.Pos.Y, Z: p.Pos.Z + dz*f}
}

// strikeAt mirrors the sim's strike buffer (strike.go, improvesNextTick)
// for a release at tick r: the first reachable tick whose next tick is no
// better strikes, or the buffer's last tick. It returns that contact's
// error, 2 for a whiff or a contact before the required bounces.
func (c *contactProfile) strikeAt(r, buf int) float64 {
	last := min(r+buf, c.n) - 1
	for j := r; j <= last; j++ {
		if c.u[j] >= 2 {
			continue
		}
		if j < last && c.u[j+1] < c.u[j] {
			continue
		}
		if c.early[j] {
			return 2
		}
		return c.u[j]
	}
	return 2
}

// bestRelease returns the release tick (from now) whose strike has the
// lowest error, and that error. A release after the ball would already
// have touched the body (no buffer live yet = a body hit) is not allowed.
func (c *contactProfile) bestRelease(buf int) (r int, u float64) {
	r, u = -1, 2.0
	for k := 0; k+buf <= c.n; k++ {
		if e := c.strikeAt(k, buf); e < u {
			r, u = k, e
		}
		if c.body[k] {
			break
		}
	}
	return r, u
}

// releaseNow reports whether the charge should be released this tick. A
// mistimed shot lets go as soon as any contact enters its (too long)
// window. Otherwise the bot releases on the tick whose buffered strike
// lands on the best contact ahead (the sim strikes on the first local
// minimum of the error, so releasing too early can lock in a worse
// contact); early ticks of TimingError make it let go that much sooner.
func (b *Bot) releaseNow(me *sim.Player, cfg *sim.Config, early, buf int) bool {
	if b.mistimed {
		return contactSoon(b.belief, me, cfg, buf-1+early+mistimeTicks, maxContactErr, b.win.min)
	}
	b.prof.profile(b.belief, me, b.tgt, cfg, 2*buf+early+1, b.win.min)
	r, u := b.prof.bestRelease(buf)
	return r >= 0 && r <= early && u <= maxContactErr
}
