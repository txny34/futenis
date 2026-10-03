package ai

import (
	"math"

	"github.com/txny34/futenis/sim"
)

const (
	// serveStandMargin puts the toss bounce safely behind the service line.
	serveStandMargin = 0.7
	// serveStandZ is |Z| of the server's spot inside its box half.
	serveStandZ = 1.5
	// serveSettleSpeed is the max speed (m/s) at which the server tosses.
	serveSettleSpeed = 0.05
	// serveAimDepth is the target depth as a fraction of ServiceLineDist.
	serveAimDepth = 0.6
	// serveMaxNoise caps the serve's aim noise (m) so it stays in the box.
	serveMaxNoise = 1.0
)

// thinkServe runs the real padel serve (docs/design/contact-v2.md §7.5):
// the server walks behind the service line, settles, presses Strike (the
// sim drops the ball), keeps charging while it falls, and releases after
// the mandatory bounce so the buffer strikes it low (foot/knee). Everyone
// else takes position.
func (b *Bot) thinkServe(s *sim.State, slot int) sim.PlayerInput {
	me := &s.Players[slot]
	if int8(slot) != s.Rally.ServerPlayer {
		b.resetCharge()
		return b.serveWait(s, slot)
	}
	if !s.Ball.Tossed {
		return b.serveSetup(s, slot)
	}
	in := b.moveTo(me, me.Pos.X, me.Pos.Z, false) // stand still
	if b.shot == 0 {
		return b.withAim(in)
	}
	b.holdTicks++
	if !s.Rally.ServeTossBounced && b.holdTicks <= maxHoldTicks {
		in.Buttons = sim.BtnStrike // keep holding: releasing now would strike before the bounce
		return b.withAim(in)
	}
	if b.shot == sim.BtnStrike {
		// The drop bounced: add Lob (re-latches ShotLob, a lob clears the
		// net from foot height), release both next tick.
		b.shot = sim.BtnStrike | sim.BtnLob
		in.Buttons = b.shot
		return b.withAim(in)
	}
	b.release(&s.Cfg)
	return b.withAim(in)
}

// serveSetup walks to the serve spot, then presses Strike once settled.
func (b *Bot) serveSetup(s *sim.State, slot int) sim.PlayerInput {
	me := &s.Players[slot]
	b.aimServe(s, slot)
	x, z := serveSpot(s, me.Team)
	in := b.moveTo(me, x, z, false)
	settled := math.Hypot(x-me.Pos.X, z-me.Pos.Z) < arriveRadius*2 &&
		math.Hypot(me.Vel.X, me.Vel.Z) < serveSettleSpeed
	if !settled || b.cooldown > 0 {
		b.resetCharge()
		return b.withAim(in)
	}
	b.shot, b.holdTicks = sim.BtnStrike, 0
	in.Buttons = sim.BtnStrike
	return b.withAim(in)
}

// serveSpot is behind the service line (so the toss, ServeTossOffset in
// front, still bounces behind it), in the ServeBox's Z half.
func serveSpot(s *sim.State, team int8) (x, z float64) {
	cfg := &s.Cfg
	depth := cfg.ServiceLineDist + cfg.ServeTossOffset + serveStandMargin
	depth = math.Min(depth, sim.CourtHalfLength-sim.PlayerRadius)
	return -forward(team) * depth, boxSign(s.Rally.ServeBox) * serveStandZ
}

// boxSign is the Z sign of ServeBox's half (box 0 = +Z, box 1 = -Z).
func boxSign(box int8) float64 {
	if box == 0 {
		return 1
	}
	return -1
}

// aimServe targets the middle of the diagonal box, with capped noise.
func (b *Bot) aimServe(s *sim.State, slot int) {
	team := s.Players[slot].Team
	noise := math.Min(b.Level.AimNoise, serveMaxNoise)
	h := sim.Hash64(b.Seed ^ uint64(s.Score[0]+s.Score[1])<<4 ^ uint64(s.Rally.ServeAttempt))
	b.aimX = forward(team) * (s.Cfg.ServiceLineDist*serveAimDepth + signed(h)*noise)
	b.aimZ = boxSign(s.Rally.ServeBox) * (sim.CourtHalfWidth/2 + signed(sim.Hash64(h))*noise)
	b.spin = 0
}

// serveWait positions non-servers: the receiver deep in the box half,
// partners on their own half.
func (b *Bot) serveWait(s *sim.State, slot int) sim.PlayerInput {
	me := &s.Players[slot]
	x, z := home(s, slot)
	if s.NumPlayers < 4 && me.Team != s.Server {
		z = boxSign(s.Rally.ServeBox) * serveStandZ
	}
	return b.withAim(b.moveTo(me, x, z, false))
}
