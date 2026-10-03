package ai

import (
	"math"

	"github.com/txny34/futenis/sim"
)

// thinkRally chases, charges and strikes when this team may touch the ball;
// otherwise it recovers / covers.
func (b *Bot) thinkRally(s *sim.State, slot int) sim.PlayerInput {
	me := &s.Players[slot]
	if s.Rally.LastTeam == me.Team {
		b.resetCharge()
		return b.recover(s, slot)
	}
	used := s.Rally.BouncesOnSide[me.Team]
	return b.chase(s, slot, bounceWindow{min: 0, max: 1 - used})
}

// chase plans an intercept, moves to it and handles the strike buttons.
func (b *Bot) chase(s *sim.State, slot int, win bounceWindow) sim.PlayerInput {
	me := &s.Players[slot]
	b.refreshPlan(s, me, win)
	if !b.plan.ok || !b.isChaser(s, slot) {
		b.resetCharge()
		return b.recover(s, slot)
	}
	fwd := forward(me.Team)
	// Positioning error is rolled per planned contact tick, not per tick.
	contact := uint64(s.Tick) + uint64(b.plan.k)
	errX := signed(sim.Hash64(b.Seed^contact)) * b.Level.PosError
	errZ := signed(sim.Hash64(b.Seed^contact^0xA5A5)) * b.Level.PosError
	tx := b.plan.ball.Pos.X - fwd*standBehind + errX
	tz := b.plan.ball.Pos.Z + errZ
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
	b.plan = findIntercept(b.belief, &s.Cfg, me, b.speedFor(me, &s.Cfg), win)
}

// isChaser decides 2v2 coordination: the teammate who reaches the planned
// contact sooner goes (ties: lower index); the other covers.
func (b *Bot) isChaser(s *sim.State, slot int) bool {
	if s.NumPlayers < 4 {
		return true
	}
	me := &s.Players[slot]
	mate := slot ^ 1
	m := &s.Players[mate]
	mine := ticksToReach(me, b.plan.ball.Pos, s.Cfg.KickReach, b.speedFor(me, &s.Cfg))
	theirs := ticksToReach(m, b.plan.ball.Pos, s.Cfg.KickReach, b.speedFor(m, &s.Cfg))
	return mine < theirs || mine == theirs && slot < mate
}

// strike presses (and latches the aim) once contact is within the charge
// window, and releases when the believed ball is in reach.
func (b *Bot) strike(s *sim.State, slot int, in sim.PlayerInput) sim.PlayerInput {
	me := &s.Players[slot]
	if b.shot == 0 {
		if b.plan.k > b.Level.ChargeTicks {
			return in
		}
		b.chooseShot(s, slot)
	}
	b.holdTicks++
	lead := 0
	if b.Level.TimingError > 0 {
		lead = int(unit(b.hash(s, slot, 3)) * float64(b.Level.TimingError+1))
	}
	ball := b.belief
	for i := 0; i < lead; i++ {
		ball, _ = stepBall(ball, &s.Cfg)
	}
	if b.holdTicks > 1 && (inReach(ball.Pos, me.Pos, &s.Cfg) || b.holdTicks > maxHoldTicks) {
		b.resetCharge()
		in.Buttons &^= sim.BtnStrike | sim.BtnLob
		return b.withAim(in)
	}
	in.Buttons = in.Buttons&^(sim.BtnStrike|sim.BtnLob) | b.shot
	if b.Level.UsesJump && b.plan.ball.Pos.Y > sim.PlayerHeight && b.plan.k <= jumpLeadTicks {
		in.Buttons |= sim.BtnJump
	}
	return b.withAim(in)
}

func (b *Bot) withAim(in sim.PlayerInput) sim.PlayerInput {
	in.TargetX, in.TargetZ, in.Spin = b.aimX, b.aimZ, b.spin
	return in
}

// chooseShot latches shot type, target and spin at press time.
func (b *Bot) chooseShot(s *sim.State, slot int) {
	me := &s.Players[slot]
	fwd := forward(me.Team)
	ox, oz := opponentsCentroid(s, me.Team)
	wide := sim.CourtHalfWidth - 1.0
	b.shot, b.spin = sim.BtnStrike, 0
	if unit(b.hash(s, slot, 4)) < b.Level.Decision {
		b.aimZ = -math.Copysign(wide, oz)
		b.aimX = fwd * (sim.CourtHalfLength - 1.5)
		switch {
		case math.Abs(ox) < 3.5:
			b.shot = sim.BtnLob
		case math.Abs(ox) > 7.5 && unit(b.hash(s, slot, 5)) < 0.3:
			b.aimX, b.spin = fwd*3, -1
		}
	} else {
		b.aimZ = signed(b.hash(s, slot, 6)) * wide
		b.aimX = fwd * (3 + unit(b.hash(s, slot, 7))*6)
	}
	b.aimX += signed(b.hash(s, slot, 8)) * b.Level.AimNoise
	b.aimZ += signed(b.hash(s, slot, 9)) * b.Level.AimNoise
	if b.Level.UsesSpin && b.shot == sim.BtnStrike && b.spin == 0 {
		b.spin = 1
	}
	b.holdTicks = 0
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

// thinkServe: the server walks behind the ball and strikes it to the
// diagonal box (wave-0 compat: the ball waits in place; once the sim sets
// Ball.Tossed the bot waits for the bounce and strikes it low). Everyone
// else takes their position.
func (b *Bot) thinkServe(s *sim.State, slot int) sim.PlayerInput {
	me := &s.Players[slot]
	if int8(slot) != s.Rally.ServerPlayer {
		b.resetCharge()
		return b.recover(s, slot)
	}
	b.aimServe(s, me.Team)
	if s.Ball.Tossed {
		win := bounceWindow{min: 1, max: 1}
		if servedBallBounced(&s.Ball) {
			win.min = 0
		}
		return b.chase(s, slot, win)
	}
	fwd := forward(me.Team)
	in := b.moveTo(me, s.Ball.Pos.X-fwd*standBehind, s.Ball.Pos.Z, false)
	if !inReach(s.Ball.Pos, me.Pos, &s.Cfg) {
		b.resetCharge()
		return b.withAim(in)
	}
	b.holdTicks++
	if b.shot != 0 && b.holdTicks > b.Level.ChargeTicks/3+1 {
		b.resetCharge()
		in.Buttons = 0
		return b.withAim(in)
	}
	b.shot = sim.BtnStrike
	in.Buttons = sim.BtnStrike
	return b.withAim(in)
}

// aimServe targets the diagonal box (ServeBox 0 = right, 1 = left).
func (b *Bot) aimServe(s *sim.State, team int8) {
	serverZ := rightSign(team)
	if s.Rally.ServeBox == 1 {
		serverZ = -serverZ
	}
	b.aimX = forward(team) * (s.Cfg.ServiceLineDist - 2)
	b.aimZ = -serverZ * sim.CourtHalfWidth / 2
	b.spin = 0
}

// servedBallBounced: a dropped serve ball moving up near the floor has
// already bounced once.
func servedBallBounced(ball *sim.Ball) bool {
	return ball.Vel.Y > 0 && ball.Pos.Y < 1
}
