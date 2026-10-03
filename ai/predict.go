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
	// reachFactor is the fraction of the reach the bot plans to use; the
	// remainder is safety margin.
	reachFactor = 0.6
	// floorBounceMinSpeed mirrors sim's resting threshold (m/s).
	floorBounceMinSpeed = 0.5
)

// intercept is a planned contact: the ball state at tick offset k.
type intercept struct {
	ok   bool
	k    int
	ball sim.Ball
}

// stepBall advances b one tick with sim.PredictBall plus a floor/wall
// bounce guard. sim.PredictBall is gravity+drag only in wave 0 (TODO in
// sim/trajectory.go); once it becomes bounce-aware the guards below simply
// never trigger. bounced reports a floor bounce on this tick.
func stepBall(b sim.Ball, cfg *sim.Config) (sim.Ball, bool) {
	b = sim.PredictBall(b, cfg, 1)
	bounced := false
	if b.Pos.Y < sim.BallRadius && b.Vel.Y < 0 {
		b.Pos.Y = sim.BallRadius
		bounced = true
		if -b.Vel.Y < floorBounceMinSpeed {
			b.Vel.Y = 0
		} else {
			b.Vel.Y = float64(-b.Vel.Y * cfg.FloorRestitution)
			b.Vel.X = float64(b.Vel.X * cfg.FloorFriction)
			b.Vel.Z = float64(b.Vel.Z * cfg.FloorFriction)
		}
	}
	if b.Pos.Y <= sim.WallHeight {
		b.Pos.X, b.Vel.X = reflectAxis(b.Pos.X, b.Vel.X, sim.CourtHalfLength-sim.BallRadius, cfg.WallRestitution)
		b.Pos.Z, b.Vel.Z = reflectAxis(b.Pos.Z, b.Vel.Z, sim.CourtHalfWidth-sim.BallRadius, cfg.WallRestitution)
	}
	return b, bounced
}

// reflectAxis bounces one coordinate off the +/-limit wall pair.
func reflectAxis(p, v, limit, rest float64) (float64, float64) {
	if p > limit {
		return limit, float64(-v * rest)
	}
	if p < -limit {
		return -limit, float64(-v * rest)
	}
	return p, v
}

// onTeamSide reports whether x lies in team's half, away from the net.
func onTeamSide(x float64, team int8) bool {
	if team == 0 {
		return x < -netClearance
	}
	return x > netClearance
}

// maxContactHeight is the highest ball the bot will plan for: the top of
// the body-part table, at least the player's height.
func maxContactHeight(cfg *sim.Config) float64 {
	h := sim.PlayerHeight
	for i := range cfg.Parts {
		h = math.Max(h, cfg.Parts[i].HMax)
	}
	return h
}

// preferredHeight is §12's Foot preference: a low contact keeps the ball
// driven flat (and, with the wave-0 compat kick, inside the glass). Head-
// height contacts are still taken, just not preferred.
func preferredHeight(cfg *sim.Config, y float64) bool {
	foot := &cfg.Parts[sim.PartFoot]
	return y >= foot.HMin && y <= foot.HMax
}

// ticksToReach estimates how many ticks player p needs to get within the
// planned reach of the ball's ground projection at speed (m/s).
func ticksToReach(p *sim.Player, ball sim.Vec3, reach, speed float64) int {
	dx, dz := ball.X-p.Pos.X, ball.Z-p.Pos.Z
	d := math.Sqrt(float64(dx*dx)+float64(dz*dz)) - reach*reachFactor
	if d <= 0 || speed <= 0 {
		return 0
	}
	return int(math.Ceil(d / float64(speed*sim.Dt)))
}

// bounceWindow bounds which floor bounces (counted on the bot's own side,
// from now) a contact may happen between: min = bounces that must happen
// first (padel serve: 1), max = bounces after which the point is lost.
type bounceWindow struct{ min, max int }

// findIntercept walks the predicted trajectory from start and returns the
// first reachable contact inside win, preferring Foot heights.
func findIntercept(start sim.Ball, cfg *sim.Config, p *sim.Player, speed float64, win bounceWindow) intercept {
	maxH := maxContactHeight(cfg)
	ball := start
	var anyHit intercept
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
		if bounces < win.min || !mine || ball.Pos.Y > maxH {
			continue
		}
		if ticksToReach(p, ball.Pos, cfg.KickReach, speed) > k {
			continue
		}
		if preferredHeight(cfg, ball.Pos.Y-sim.BallRadius) {
			return intercept{ok: true, k: k, ball: ball}
		}
		if !anyHit.ok {
			anyHit = intercept{ok: true, k: k, ball: ball}
		}
	}
	return anyHit
}

// inReach mirrors the sim's contact check (wave-0 compat: KickReach from a
// vertical segment 0..PlayerHeight at the player's XZ) with a safety factor.
func inReach(ball sim.Vec3, pos sim.Vec3, cfg *sim.Config) bool {
	y := math.Max(0, math.Min(ball.Y, sim.PlayerHeight))
	d := ball.Sub(sim.Vec3{X: pos.X, Y: y, Z: pos.Z}).Len()
	return d <= cfg.KickReach*0.9
}
