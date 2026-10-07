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
	// serveDepthSpread is the +/- spread of the serve depth fraction.
	serveDepthSpread = 0.2
	// serveMinDepth keeps an in-box serve this far (m) past the net.
	serveMinDepth = 2.0
	// serveFaultMin/serveFaultSpan: a fault serve misses the box line by
	// serveFaultMin..serveFaultMin+serveFaultSpan metres.
	serveFaultMin  = 0.4
	serveFaultSpan = 0.8
)

// servePlan holds one serve attempt's rolls (deterministic per attempt).
type servePlan struct {
	delay int // extra settled ticks before the toss
}

// thinkServe runs the real padel serve (docs/design/contact-v2.md §7.5):
// the server walks behind the service line, settles, presses Strike (the
// sim drops the ball), keeps charging while it falls, and releases after
// the mandatory bounce so the buffer strikes it low (foot/knee). Padel has
// no lob serve: the server never presses Lob. Everyone else takes position.
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
	bounced := s.Rally.ServeTossBounced || b.holdTicks > maxHoldTicks
	if !bounced {
		in.Buttons = b.shot // releasing now would strike before the bounce
		return b.withAim(in)
	}
	b.release(&s.Cfg)
	return b.withAim(in)
}

// serveSetup walks to the serve spot, waits its toss delay once settled,
// then presses Strike.
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
	if b.holdTicks < b.serve.delay {
		b.holdTicks++ // toss timing variety
		return b.withAim(in)
	}
	b.shot, b.holdTicks = sim.BtnStrike, 0
	in.Buttons = sim.BtnStrike
	return b.withAim(in)
}

// serveSpot is behind the service line (so the toss, ServeTossOffset in
// front, still bounces behind it), on the half diagonally opposite the
// target box: the serve goes crosscourt.
func serveSpot(s *sim.State, team int8) (x, z float64) {
	cfg := &s.Cfg
	depth := cfg.ServiceLineDist + cfg.ServeTossOffset + serveStandMargin
	depth = math.Min(depth, sim.CourtHalfLength-sim.PlayerRadius)
	return -forward(team) * depth, -boxZ(s) * serveStandZ
}

// boxZ is the Z sign of the target box (sim's side convention: box 0 = the
// receiver's right, box 1 = the receiver's left).
func boxZ(s *sim.State) float64 { return sim.ServeBoxZSign(s.Server, s.Rally.ServeBox) }

// aimServe rolls this attempt's serve (stable for the whole attempt): a
// target inside the diagonal box, Level.ServeLine from its lines around
// Level.ServeDepth -- or, Level.ServeFault of the time, just outside it --
// plus the toss delay.
func (b *Bot) aimServe(s *sim.State, slot int) {
	lv, cfg := &b.Level, &s.Cfg
	played := s.Score[0] + s.Score[1] + 8*(s.Games[0]+s.Games[1]+s.Sets[0]+s.Sets[1])
	h := sim.Hash64(b.Seed*0x9E3779B97F4A7C15 ^ uint64(played)<<4 ^ uint64(s.Rally.ServeAttempt))
	x := cfg.ServiceLineDist * (lv.ServeDepth + signed(sim.Hash64(h^1))*serveDepthSpread)
	x = math.Max(serveMinDepth, math.Min(cfg.ServiceLineDist-lv.ServeLine, x))
	half := sim.CourtHalfWidth / 2
	z := half + signed(sim.Hash64(h^2))*(half-lv.ServeLine)
	if unit(sim.Hash64(h^3)) < lv.ServeFault {
		miss := serveFaultMin + unit(sim.Hash64(h^4))*serveFaultSpan
		if sim.Hash64(h^5)&1 == 0 {
			x = cfg.ServiceLineDist + miss // long
		} else {
			z = -miss // wrong box
		}
	}
	b.aimX, b.aimZ, b.spin = forward(s.Players[slot].Team)*x, boxZ(s)*z, 0
	b.serve.delay = int(unit(sim.Hash64(h^7)) * float64(lv.ServeTossJitter+1))
}

// serveWait positions non-servers: the 1v1 receiver in the target box, the
// server's 2v2 partner on the half the server left free, receivers on their
// own slot's half.
func (b *Bot) serveWait(s *sim.State, slot int) sim.PlayerInput {
	me := &s.Players[slot]
	x, z := home(s, slot)
	switch {
	case s.NumPlayers < 4 && me.Team != s.Server:
		z = boxZ(s) * serveStandZ
	case s.NumPlayers >= 4 && me.Team == s.Server:
		z = boxZ(s) * sim.CourtHalfWidth / 2
	}
	return b.withAim(b.moveTo(me, x, z, false))
}
