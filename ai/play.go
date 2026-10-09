package ai

import (
	"math"

	"github.com/txny34/futenis/sim"
)

// thinkRally chases, charges and strikes when this team may touch the ball
// (one touch per TEAM per crossing, 2v2 partner included); otherwise it
// recovers / covers.
func (b *Bot) thinkRally(s *sim.State, slot int) sim.PlayerInput {
	me := &s.Players[slot]
	if s.Rally.LastTeam == me.Team || s.Rally.LastTeam < 0 {
		b.resetCharge()
		return b.recover(s, slot)
	}
	if b.missed && s.Rally.ShotCount == b.missedShot {
		// This bot whiffed this ball (a mistimed shot): it does not get a
		// second try at the same crossing.
		b.resetCharge()
		return b.recover(s, slot)
	}
	b.missed = false
	used := s.Rally.BouncesOnSide[me.Team]
	win := bounceWindow{min: 0, max: 1 - used}
	if !s.Rally.ServeBoxPending {
		return b.chase(s, slot, win)
	}
	win.min = 1 // the serve must bounce in the box before it is returned
	return b.dodgeServe(s, slot, b.chase(s, slot, win))
}

// chase plans an intercept, moves to it and handles the strike buttons.
func (b *Bot) chase(s *sim.State, slot int, win bounceWindow) sim.PlayerInput {
	me := &s.Players[slot]
	if b.cooldown > 0 {
		// Strike buffer live: hold position, never re-press.
		return b.withAim(b.moveTo(me, b.tgt.X, b.tgt.Z, false))
	}
	b.win = win
	b.refreshPlan(s, me, win)
	if !b.plan.ok || !b.isChaser(s, slot) {
		in := b.recover(s, slot)
		if b.shot != 0 && s.Rally.ServeBoxPending {
			// Letting go would arm a live strike buffer onto the
			// unbounced serve (a volley, §21.6): keep holding.
			in.Buttons |= b.shot
			return b.withAim(in)
		}
		b.resetCharge()
		return in
	}
	// Positioning error is rolled per planned contact tick, not per tick.
	b.tgt = b.plan.stand.Add(b.posError(uint64(s.Tick) + uint64(b.plan.k)))
	in := b.moveTo(me, b.tgt.X, b.tgt.Z, true)
	return b.strike(s, slot, in)
}

// posError is the Level.PosError miss at the planned contact. On a serve
// return (b.win.min > 0) it only slides along the ball's path: a sideways
// miss could put the body on the falling serve before it bounces (§21.6).
func (b *Bot) posError(contact uint64) sim.Vec3 {
	ex := signed(sim.Hash64(b.Seed^contact)) * b.Level.PosError
	ez := signed(sim.Hash64(b.Seed^contact^0xA5A5)) * b.Level.PosError
	v := b.plan.ball.Vel
	l := math.Hypot(v.X, v.Z)
	if b.win.min == 0 || l < 1e-6 {
		return sim.Vec3{X: ex, Z: ez}
	}
	return sim.Vec3{X: v.X / l * ex, Z: v.Z / l * ex}
}

// refreshPlan recomputes the intercept every replanEvery ticks.
func (b *Bot) refreshPlan(s *sim.State, me *sim.Player, win bounceWindow) {
	b.planAge++
	if b.plan.ok && b.planAge < replanEvery && b.plan.k > 1 {
		b.plan.k--
		return
	}
	b.planAge = 0
	speed := b.speedFor(me, &s.Cfg)
	// A serve return (win.min > 0) keeps the side-on stand-off: standing on
	// the ball's path would take the pre-bounce ball on the body.
	if e := b.Level.IdealErr; e > 0 && win.min == 0 {
		b.plan = findIntercept(b.belief, &s.Cfg, me, speed, e, e*idealOffset, win)
		if b.plan.ok {
			return
		}
	}
	b.plan = findIntercept(b.belief, &s.Cfg, me, speed, maxContactErr, lateral, win)
}

// isChaser decides 2v2 coordination: the teammate who reaches the planned
// stand spot sooner goes (ties: lower index); the other covers.
func (b *Bot) isChaser(s *sim.State, slot int) bool {
	if s.NumPlayers < 4 {
		return true
	}
	mate := slot ^ 1
	me, m := &s.Players[slot], &s.Players[mate]
	cfg := &s.Cfg
	mine := ticksToReach(me, standSpot(me, b.plan.ball, b.plan.off), b.speedFor(me, cfg), cfg.PlayerAccel)
	theirs := ticksToReach(m, standSpot(m, b.plan.ball, b.plan.off), b.speedFor(m, cfg), cfg.PlayerAccel)
	return mine < theirs || mine == theirs && slot < mate
}

// mistimeTicks is how much earlier than its buffer window a mistimed shot
// is released: far enough that the strike buffer expires before contact.
const mistimeTicks = 30

const (
	// pressureSpeed is the incoming horizontal ball speed (m/s) above which
	// the bot starts to feel pressure (see missChance).
	pressureSpeed = 16.0
	// pressureSpan is the extra speed (m/s) to reach full pressure.
	pressureSpan = 8.0
	// pressureMissGain is the extra MissChance multiple at full pressure.
	pressureMissGain = 4.0
)

// strike presses (latching aim) once contact is within the charge window,
// and releases as soon as a contact falls inside the strike buffer window
// (sim's improvesNextTick then picks the best tick of that window).
func (b *Bot) strike(s *sim.State, slot int, in sim.PlayerInput) sim.PlayerInput {
	me := &s.Players[slot]
	cfg := &s.Cfg
	buf := ticksOfSec(cfg.StrikeBufferTime)
	if b.shot == 0 {
		if !b.aimed {
			if b.plan.k > ticksOfSec(cfg.ChargeMaxTime)+buf {
				return in
			}
			b.chooseShot(s, slot)
		}
		if b.plan.k > b.chargeWant+buf {
			return b.withAim(in)
		}
		b.shot, b.holdTicks = b.pendingShot, 0
	}
	b.holdTicks++
	early := 0
	if b.Level.TimingError > 0 {
		early = int(unit(b.hash(s, slot, 3)) * float64(b.Level.TimingError+1))
	}
	// Never release before the serve bounces: the sim's buffer strikes on
	// any reachable tick, and touching the serve first loses the point.
	canRelease := b.holdTicks > 1 && !s.Rally.ServeBoxPending
	if canRelease && (b.releaseNow(me, cfg, early, buf) || b.holdTicks > maxHoldTicks) {
		b.missed, b.missedShot = b.mistimed, s.Rally.ShotCount
		b.release(cfg)
		in.Buttons &^= sim.BtnStrike | sim.BtnLob
		return b.withAim(in)
	}
	in.Buttons = in.Buttons&^(sim.BtnStrike|sim.BtnLob) | b.shot
	head := &cfg.Parts[sim.PartHead]
	if b.Level.UsesJump && b.plan.ball.Pos.Y > head.HMax && b.plan.k <= jumpLeadTicks {
		in.Buttons |= sim.BtnJump
	}
	return b.withAim(in)
}

func (b *Bot) withAim(in sim.PlayerInput) sim.PlayerInput {
	in.TargetX, in.TargetZ, in.Spin = b.aimX, b.aimZ, b.spin
	return in
}

// chooseShot latches shot type, target and spin at press time: open
// corner away from the opponents, deep; a lob over net-rushers; sometimes
// a short backspin drop against a deep opponent. Low-decision bots pick a
// random spot. Aim noise is added on top.
func (b *Bot) chooseShot(s *sim.State, slot int) {
	me := &s.Players[slot]
	fwd := forward(me.Team)
	ox, oz := opponentsCentroid(s, me.Team)
	wide := sim.CourtHalfWidth - b.Level.LineMargin
	shot := sim.BtnStrike
	b.spin = 0
	b.mistimed = unit(b.hash(s, slot, 9)) < b.missChance()
	if unit(b.hash(s, slot, 4)) < b.Level.Decision {
		b.aimZ = -math.Copysign(wide, oz)
		b.aimX = fwd * (sim.CourtHalfLength - b.Level.LineMargin - 0.5)
		switch {
		case math.Abs(ox) < 3.5:
			shot = sim.BtnLob
		case math.Abs(ox) > 7.5 && unit(b.hash(s, slot, 5)) < b.Level.DropChance:
			b.aimX, b.spin = fwd*3, -1
		}
	} else {
		b.aimZ = signed(b.hash(s, slot, 6)) * wide
		b.aimX = fwd * (3 + unit(b.hash(s, slot, 7))*5)
	}
	b.aimX += signed(b.hash(s, slot, 8)) * b.Level.AimNoise
	b.aimZ += signed(b.hash(s, slot, 9)) * b.Level.AimNoise
	if b.Level.UsesSpin && shot == sim.BtnStrike && b.spin == 0 {
		b.spin = 1
	}
	b.pendingShot, b.aimed = shot, true
	b.chargeWant = b.chargeFor(s, slot, shot)
}

// missChance is Level.MissChance raised under pressure: a fast incoming
// ball (typically an opponent's PERFECT contact, sim's perfectSpeedBonus)
// is harder to meet cleanly, up to pressureMissGain times more misses. This
// is what turns a PERFECT contact into a point between strong bots.
func (b *Bot) missChance() float64 {
	v := math.Hypot(b.belief.Vel.X, b.belief.Vel.Z)
	p := math.Max(0, math.Min(1, (v-pressureSpeed)/pressureSpan))
	return math.Min(1, b.Level.MissChance*(1+pressureMissGain*p))
}

// chargeFor picks how long to charge: a lob barely needs any; a drive
// gets the fastest speed whose (drag-free) arc still clears the net by
// netMargin from the planned contact to the latched target -- the sim's
// solver flies a too-fast drive flat into the net.
func (b *Bot) chargeFor(s *sim.State, slot int, shot uint8) int {
	cfg := &s.Cfg
	maxT := ticksOfSec(cfg.ChargeMaxTime)
	if shot == sim.BtnLob {
		return maxT / 4
	}
	from := b.plan.ball.Pos
	part, _ := sim.BestPart(cfg, from.Y, b.plan.off, false)
	if part == sim.PartNone {
		part = sim.PartFoot
	}
	ps := &cfg.Parts[part]
	d := math.Hypot(b.aimX-from.X, b.aimZ-from.Z)
	dn := math.Abs(from.X) / math.Max(math.Abs(b.aimX-from.X), 1e-6) * d
	need := sim.NetHeight + sim.BallRadius + b.Level.NetMargin - from.Y*(d-dn)/math.Max(d, 1e-6)
	v := ps.SpeedMax
	if need > 0 && dn < d {
		v = math.Sqrt(cfg.Gravity / 2 * dn * (d - dn) / need)
	}
	frac := (v - ps.SpeedMin) / math.Max(ps.SpeedMax-ps.SpeedMin, 1e-6)
	frac += signed(b.hash(s, slot, 10)) * b.Level.ChargeError
	frac = math.Max(0, math.Min(1, frac))
	return int(frac * float64(maxT))
}

// opponentsCentroid is the mean position of the active opponents.
func opponentsCentroid(s *sim.State, team int8) (x, z float64) {
	n := 0.0
	for i := int8(0); i < s.NumPlayers; i++ {
		if p := &s.Players[i]; p.Team != team {
			x, z, n = x+p.Pos.X, z+p.Pos.Z, n+1
		}
	}
	if n == 0 {
		return 0, 0
	}
	return x / n, z / n
}
