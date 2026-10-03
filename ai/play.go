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
	used := s.Rally.BouncesOnSide[me.Team]
	win := bounceWindow{min: 0, max: 1 - used}
	if s.Rally.ServeBoxPending {
		win.min = 1 // the serve must bounce in the box before it is returned
	}
	return b.chase(s, slot, win)
}

// chase plans an intercept, moves to it and handles the strike buttons.
func (b *Bot) chase(s *sim.State, slot int, win bounceWindow) sim.PlayerInput {
	me := &s.Players[slot]
	if b.cooldown > 0 {
		// Strike buffer live: hold position, never re-press.
		return b.withAim(b.moveTo(me, b.plan.stand.X, b.plan.stand.Z, false))
	}
	b.win = win
	b.refreshPlan(s, me, win)
	if !b.plan.ok || !b.isChaser(s, slot) {
		b.resetCharge()
		return b.recover(s, slot)
	}
	// Positioning error is rolled per planned contact tick, not per tick.
	contact := uint64(s.Tick) + uint64(b.plan.k)
	tx := b.plan.stand.X + signed(sim.Hash64(b.Seed^contact))*b.Level.PosError
	tz := b.plan.stand.Z + signed(sim.Hash64(b.Seed^contact^0xA5A5))*b.Level.PosError
	in := b.moveTo(me, tx, tz, true)
	return b.strike(s, slot, in)
}

// refreshPlan recomputes the intercept every replanEvery ticks.
func (b *Bot) refreshPlan(s *sim.State, me *sim.Player, win bounceWindow) {
	b.planAge++
	if b.plan.ok && b.planAge < replanEvery && b.plan.k > 1 {
		b.plan.k--
		return
	}
	b.planAge = 0
	b.plan = findIntercept(b.belief, &s.Cfg, me, b.speedFor(me, &s.Cfg), maxContactErr, win)
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
	mine := ticksToReach(me, standSpot(me, b.plan.ball), b.speedFor(me, cfg), cfg.PlayerAccel)
	theirs := ticksToReach(m, standSpot(m, b.plan.ball), b.speedFor(m, cfg), cfg.PlayerAccel)
	return mine < theirs || mine == theirs && slot < mate
}

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
	window := buf - 1
	if b.Level.TimingError > 0 {
		window += int(unit(b.hash(s, slot, 3)) * float64(b.Level.TimingError+1))
	}
	if b.holdTicks > 1 && (contactSoon(b.belief, me, cfg, window, maxContactErr, b.win.min) || b.holdTicks > maxHoldTicks) {
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
	part, _ := bestErr(cfg, from.Y, lateral, false)
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
