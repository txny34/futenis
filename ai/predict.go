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
	// floorBounceMinSpeed mirrors sim's resting threshold (m/s).
	floorBounceMinSpeed = 0.5
	// lateral is the side-on stand-off (m) between the player's axis and
	// the ball at contact: inside every low part's Reach, outside the body
	// capsule (PlayerRadius+BallRadius = 0.46), so the ball meets the foot
	// before it can bounce off the body (EvBodyHit = a touch).
	lateral = 0.55
	// maxContactErr is the worst part error the bot plans/releases for.
	maxContactErr = 0.95
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
	serve  bool
	errMax float64
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

// Mirrors of sim/physics.go's unexported bounce constants (keep in sync).
const (
	floorSpinFrictionMin = 0.4
	floorSpinFrictionMax = 1.05
	floorSpinRestMax     = 0.95
	wallSpinRestMax      = 1.0
	netBounceDamping     = 0.3
)

// stepBall advances b one tick: sim.PredictBall (gravity+drag; still
// bounce-less, see the TODO in sim/trajectory.go) plus a mirror of
// stepBall's net/floor/wall resolution with the §10 spin model. bounced
// reports a floor bounce on this tick.
func stepBall(b sim.Ball, cfg *sim.Config) (sim.Ball, bool) {
	prev := b.Pos
	b = sim.PredictBall(b, cfg, 1)
	b = predictNet(b, prev)
	b, bounced := predictFloor(b, cfg)
	inside := math.Abs(prev.X) <= sim.CourtHalfLength && math.Abs(prev.Z) <= sim.CourtHalfWidth
	if b.Pos.Y <= sim.WallHeight && inside {
		b = predictWalls(b, cfg)
	}
	return b, bounced
}

func predictNet(b sim.Ball, prev sim.Vec3) sim.Ball {
	if (prev.X < 0) == (b.Pos.X < 0) {
		return b
	}
	t := (0 - prev.X) / (b.Pos.X - prev.X)
	if prev.Y+float64((b.Pos.Y-prev.Y)*t) >= sim.NetHeight+sim.BallRadius {
		return b
	}
	sign := 1.0
	if prev.X < 0 {
		sign = -1
	}
	b.Pos.X = sign * sim.BallRadius
	b.Vel.X = float64(-b.Vel.X * netBounceDamping)
	return b
}

func predictFloor(b sim.Ball, cfg *sim.Config) (sim.Ball, bool) {
	if b.Pos.Y >= sim.BallRadius {
		b.Resting = false
		return b, false
	}
	b.Pos.Y = sim.BallRadius
	if b.Vel.Y >= 0 {
		return b, false
	}
	incoming := -b.Vel.Y
	if incoming < floorBounceMinSpeed {
		b.Vel.Y = 0
		was := b.Resting
		b.Resting = true
		return b, !was
	}
	b.Resting = false
	fric := clampF(float64(cfg.FloorFriction+cfg.SpinFloorFriction*b.Spin), floorSpinFrictionMin, floorSpinFrictionMax)
	rest := clampF(float64(cfg.FloorRestitution*(1-cfg.SpinFloorRest*b.Spin)), 0, floorSpinRestMax)
	b.Vel.Y = incoming * rest
	b.Vel.X *= fric
	b.Vel.Z *= fric
	b.Spin *= cfg.SpinDecay
	return b, true
}

func predictWalls(b sim.Ball, cfg *sim.Config) sim.Ball {
	restW := clampF(float64(cfg.WallRestitution*(1+cfg.SpinWallRest*math.Min(b.Spin, 0))), 0, wallSpinRestMax)
	var hx, hz bool
	b.Pos.X, b.Vel.X, hx = reflectAxis(b.Pos.X, b.Vel.X, sim.CourtHalfLength-sim.BallRadius, restW)
	b.Pos.Z, b.Vel.Z, hz = reflectAxis(b.Pos.Z, b.Vel.Z, sim.CourtHalfWidth-sim.BallRadius, restW)
	if hx || hz {
		if b.Spin < 0 {
			b.Vel.Y -= float64(cfg.SpinWallDrop * (-b.Spin))
		}
		b.Spin *= cfg.SpinDecay
	}
	return b
}

func clampF(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// reflectAxis bounces one coordinate off the +/-limit wall pair.
func reflectAxis(p, v, limit, rest float64) (float64, float64, bool) {
	if p > limit {
		return limit, float64(-v * rest), true
	}
	if p < -limit {
		return -limit, float64(-v * rest), true
	}
	return p, v, false
}

// onTeamSide reports whether x lies in team's half, away from the net.
func onTeamSide(x float64, team int8) bool {
	if team == 0 {
		return x < -netClearance
	}
	return x > netClearance
}

// standSpot is where to stand for a side-on contact with ball: lateral
// metres off the ball's horizontal path, on the side closer to the player
// (falls back to "behind the ball" for a near-vertical ball).
func standSpot(p *sim.Player, ball sim.Ball) sim.Vec3 {
	vx, vz := ball.Vel.X, ball.Vel.Z
	l := math.Sqrt(float64(vx*vx) + float64(vz*vz))
	if l < 0.5 {
		return sim.Vec3{X: ball.Pos.X - forward(p.Team)*lateral, Z: ball.Pos.Z}
	}
	nx, nz := -vz/l, vx/l
	if float64((p.Pos.X-ball.Pos.X)*nx)+float64((p.Pos.Z-ball.Pos.Z)*nz) < 0 {
		nx, nz = -nx, -nz
	}
	s := sim.Vec3{X: ball.Pos.X + nx*lateral, Z: ball.Pos.Z + nz*lateral}
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
// stand-off is <= errMax, preferring Foot/Knee contacts (more power and
// accuracy than chest/shoulder).
func findIntercept(start sim.Ball, cfg *sim.Config, p *sim.Player, speed, errMax float64, win bounceWindow) intercept {
	ball := start
	var fallback intercept
	bounces := 0
	for k := 1; k <= horizonTicks; k++ {
		var b bool
		ball, b = stepBall(ball, cfg)
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
		part, u := bestErr(cfg, ball.Pos.Y, lateral, false)
		if u > errMax {
			continue
		}
		stand := standSpot(p, ball)
		if ticksToReach(p, stand, speed, cfg.PlayerAccel)+arriveSlackTicks > k {
			continue
		}
		hit := intercept{ok: true, k: k, ball: ball, stand: stand, errMax: errMax}
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
