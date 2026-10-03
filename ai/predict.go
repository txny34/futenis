package ai

import (
	"math"

	"github.com/txny34/futenis/sim"
)

const (
	// horizonTicks bounds the intercept lookahead (2.5 s).
	horizonTicks = sim.TickRate * 5 / 2
	// beliefTolerance is how far (m) the real ball may drift from the bot's
	// belief before the bot "notices" (starts its reaction countdown).
	beliefTolerance = 0.15
	// netClearance keeps the intercept search away from the net plane.
	netClearance = 0.4
	// lateral is the side-on stand-off (m) between the player's axis and
	// the ball at contact: inside every low part's Reach, outside the body
	// capsule (PlayerRadius+BallRadius = 0.46), so the ball meets the foot
	// before it can bounce off the body (EvBodyHit = a touch).
	lateral = 0.55
	// maxContactErr is the worst part error the bot plans/releases for.
	maxContactErr = 0.95
	// idealOffset converts a Level.IdealErr into the planned side-on
	// stand-off (m): standing at IdealErr*idealOffset keeps the reach term
	// of the contact error (dH/Reach) well inside the target error.
	idealOffset = 0.5
	// arriveSlackTicks is the safety margin when judging reachability.
	arriveSlackTicks = 3
)

// intercept is a planned contact: the ball state at tick offset k and the
// spot (XZ) the player should stand on.
type intercept struct {
	ok     bool
	k      int
	ball   sim.Ball
	stand  sim.Vec3
	errMax float64
	off    float64 // planned side-on stand-off (m)
}

// ticksOfSec mirrors sim's unexported ticksOf (seconds -> ticks, >= 1).
func ticksOfSec(sec float64) int {
	t := int(math.Round(sec * sim.TickRate))
	if t < 1 {
		return 1
	}
	return t
}

// partErr mirrors sim's unexported partError (docs/design/contact-v2.md
// §7.2): 0 = perfect, >= 1 = out of reach.
func partErr(ps *sim.PartSpec, hRel, dH float64) float64 {
	if hRel < ps.HMin || hRel > ps.HMax || dH > ps.Reach {
		return 2
	}
	half := ps.HMax - ps.HIdeal
	if hRel < ps.HIdeal {
		half = ps.HIdeal - ps.HMin
	}
	if half == 0 {
		half = 1e-9
	}
	return math.Max(math.Abs(hRel-ps.HIdeal)/half, dH/ps.Reach)
}

// bestErr mirrors sim's bestPartConstrained without the bicycle (the bot
// never plans bicycle kicks); serveOnly limits it to foot/knee (§7.5).
func bestErr(cfg *sim.Config, hRel, dH float64, serveOnly bool) (sim.BodyPart, float64) {
	best, bestU := sim.PartNone, 2.0
	for i := sim.PartFoot; i < sim.PartBicycle; i++ {
		if serveOnly && i > sim.PartKnee {
			break
		}
		if u := partErr(&cfg.Parts[i], hRel, dH); u < bestU {
			best, bestU = i, u
		}
	}
	return best, bestU
}

// stepBall advances b one tick with sim.PredictBall (which resolves
// net/floor/wall exactly like the sim) and reports whether that tick was a
// floor bounce, read off the returned state: the ball sits on the floor and
// either rebounded (falling before, rising after) or just came to rest
// (edge-triggered, like the sim's EvBounceFloor).
func stepBall(b sim.Ball, cfg *sim.Config) (sim.Ball, bool) {
	next := sim.PredictBall(b, cfg, 1)
	onFloor := next.Pos.Y <= sim.BallRadius
	rebound := b.Vel.Y <= 0 && next.Vel.Y > 0
	settled := next.Resting && !b.Resting
	return next, onFloor && (rebound || settled)
}

// onTeamSide reports whether x lies in team's half, away from the net.
func onTeamSide(x float64, team int8) bool {
	if team == 0 {
		return x < -netClearance
	}
	return x > netClearance
}

// standSpot is where to stand for a side-on contact with ball: off
// metres off the ball's horizontal path, on the side closer to the player
// (falls back to "behind the ball" for a near-vertical ball).
func standSpot(p *sim.Player, ball sim.Ball, off float64) sim.Vec3 {
	vx, vz := ball.Vel.X, ball.Vel.Z
	l := math.Sqrt(float64(vx*vx) + float64(vz*vz))
	if l < 0.5 {
		return sim.Vec3{X: ball.Pos.X - forward(p.Team)*off, Z: ball.Pos.Z}
	}
	nx, nz := -vz/l, vx/l
	if float64((p.Pos.X-ball.Pos.X)*nx)+float64((p.Pos.Z-ball.Pos.Z)*nz) < 0 {
		nx, nz = -nx, -nz
	}
	s := sim.Vec3{X: ball.Pos.X + nx*off, Z: ball.Pos.Z + nz*off}
	minX, maxX := teamXRange(p.Team)
	s.X = math.Max(minX, math.Min(maxX, s.X))
	zl := sim.CourtHalfWidth - sim.PlayerRadius
	s.Z = math.Max(-zl, math.Min(zl, s.Z))
	return s
}

// teamXRange mirrors sim's player X clamp for team.
func teamXRange(team int8) (float64, float64) {
	if team == 0 {
		return -sim.CourtHalfLength + sim.PlayerRadius, -sim.PlayerRadius
	}
	return sim.PlayerRadius, sim.CourtHalfLength - sim.PlayerRadius
}

// ticksToReach estimates ticks for p to reach spot: an acceleration ramp
// from the current velocity, then cruise at speed.
func ticksToReach(p *sim.Player, spot sim.Vec3, speed, accel float64) int {
	dx, dz := spot.X-p.Pos.X, spot.Z-p.Pos.Z
	d := math.Sqrt(float64(dx*dx) + float64(dz*dz))
	if d < 0.05 || speed <= 0 {
		return 0
	}
	ramp := 0.0
	if accel > 0 {
		ramp = speed / (2 * accel)
	}
	return int(math.Ceil((d/speed + ramp) * sim.TickRate))
}

// bounceWindow bounds which floor bounces (counted on the bot's own side,
// from now) a contact may happen between: min = bounces that must happen
// first, max = bounces after which the point is lost.
type bounceWindow struct{ min, max int }

// findIntercept walks the predicted trajectory and returns the first
// reachable contact inside win whose best part error at the side-on
// stand-off off is <= errMax, preferring Foot/Knee contacts (more power and
// accuracy than chest/shoulder).
func findIntercept(start sim.Ball, cfg *sim.Config, p *sim.Player, speed, errMax, off float64, win bounceWindow) intercept {
	ball := start
	var fallback intercept
	var path [horizonTicks + 1]sim.Vec3
	bounces := 0
	for k := 1; k <= horizonTicks; k++ {
		var b bool
		ball, b = stepBall(ball, cfg)
		path[k] = ball.Pos
		mine := onTeamSide(ball.Pos.X, p.Team)
		if b && mine {
			bounces++
		}
		if bounces > win.max {
			break
		}
		if bounces < win.min || !mine {
			continue
		}
		part, u := bestErr(cfg, ball.Pos.Y, off, false)
		if u > errMax {
			continue
		}
		stand := standSpot(p, ball, off)
		if ticksToReach(p, stand, speed, cfg.PlayerAccel)+arriveSlackTicks > k {
			continue
		}
		if off < lateral && !clearPath(&path, k-ticksOfSec(cfg.StrikeBufferTime)+2, stand) {
			continue
		}
		hit := intercept{ok: true, k: k, ball: ball, stand: stand, errMax: errMax, off: off}
		if part <= sim.PartKnee {
			return hit
		}
		if !fallback.ok {
			fallback = hit
		}
	}
	return fallback
}

// contactSoon reports whether, within window ticks, the believed ball
// reaches a contact (error <= maxErr) for player p extrapolated with its
// current velocity -- the release test for the strike buffer. minBounces
// floor bounces must happen first (serve return); since the sim's buffer
// strikes on any reachable tick, a window that would reach the ball BEFORE
// those bounces is reported as not-yet (false).
func contactSoon(ball sim.Ball, p *sim.Player, cfg *sim.Config, window int, maxErr float64, minBounces int) bool {
	bounces := 0
	for j := 1; j <= window; j++ {
		var b bool
		ball, b = stepBall(ball, cfg)
		if b {
			bounces++
		}
		pos := p.Pos.AddScaled(p.Vel, float64(j)*sim.Dt)
		dH := math.Hypot(ball.Pos.X-pos.X, ball.Pos.Z-pos.Z)
		_, u := bestErr(cfg, ball.Pos.Y-p.Pos.Y, dH, false)
		if bounces < minBounces {
			if u < 2 {
				return false
			}
			continue
		}
		if u <= maxErr {
			return true
		}
	}
	return false
}

// clearPath reports whether the ball stays off a body capsule standing at
// stand on every path tick before upTo: an ideal (close) stand-off is only
// worth planning if the ball does not hit the body before the strike
// buffer can be live (the buffer makes the body immune, §7.4).
func clearPath(path *[horizonTicks + 1]sim.Vec3, upTo int, stand sim.Vec3) bool {
	const r = sim.PlayerRadius + sim.BallRadius + bodyMargin
	for j := 1; j < upTo; j++ {
		q := path[j]
		if q.Y < sim.PlayerHeight+sim.BallRadius && math.Hypot(q.X-stand.X, q.Z-stand.Z) < r {
			return false
		}
	}
	return true
}

// bodyMargin pads the body capsule radius in clearPath (m).
const bodyMargin = 0.05
