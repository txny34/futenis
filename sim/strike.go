package sim

import "math"

// candidate is one player's resolved contact attempt for this tick, kept in
// a fixed-size array (no slice) so applyStrikes stays zero-alloc.
type candidate struct {
	ok   bool
	part BodyPart
	u    float64
}

// applyStrikes is Step's single entry point for contact resolution
// (docs/design/contact-v2.md §7.2), replacing the wave-0 applyKicks/
// performKick compat shim: it advances every active player's charge/buffer
// state, resolves at most one body-part contact for the whole tick (even in
// 2v2, where up to 4 players may have a live buffer at once), and appends
// EvWhiff/EvToss/EvKick as appropriate.
func applyStrikes(s *State, inputs [MaxPlayers]PlayerInput, ev []Event) []Event {
	var cand [MaxPlayers]candidate
	for i := int8(0); i < s.NumPlayers; i++ {
		ev = updateCharge(s, i, inputs[i], ev)

		p := &s.Players[i]
		if p.Buffer == 0 || !canKick(s, i) {
			continue
		}
		p.Buffer--

		hRel := s.Ball.Pos.Y - p.Pos.Y
		dH := math.Hypot(s.Ball.Pos.X-p.Pos.X, s.Ball.Pos.Z-p.Pos.Z)
		bicycleOK := !p.OnGround && p.BufCharge >= 0.9
		serveOnly := s.Phase == PhaseServe

		part, u := bestPartConstrained(&s.Cfg, hRel, dH, bicycleOK, serveOnly)
		if part == PartNone {
			if p.Buffer == 0 {
				ev = append(ev, whiffEvent(s, i))
			}
			continue
		}
		if p.Buffer > 0 && improvesNextTick(s, p, u, bicycleOK, serveOnly) {
			continue
		}
		cand[i] = candidate{ok: true, part: part, u: u}
	}

	w := pickStriker(s, &cand)
	if w < 0 {
		return ev
	}
	s.Players[w].Buffer = 0
	return strike(s, w, cand[w].part, cand[w].u, ev)
}

// updateCharge handles one player's press/release edges for Strike/Lob
// (docs/design/contact-v2.md §7.1): press latches ShotType and starts the
// charge (cancelling any still-buffered old intention); the charge
// saturates at cfg.chargeMaxTicks while held; release freezes Buffer/
// BufCharge/BufTarget/BufSpin and resets the charge. During PhaseServe, the
// server's Strike press also releases/drops the ball for the serve attempt
// (§7.5).
func updateCharge(s *State, i int8, in PlayerInput, ev []Event) []Event {
	p := &s.Players[i]
	cfg := &s.Cfg

	pressStrike := pressed(p, in, BtnStrike)
	pressLob := pressed(p, in, BtnLob)

	if pressStrike && s.Phase == PhaseServe && i == s.Rally.ServerPlayer && !s.Ball.Tossed {
		ev = append(ev, tossServe(s))
	}

	if pressStrike || pressLob {
		p.Buffer = 0 // re-press cancels any still-buffered old intention
		p.ChargeTicks = 0
		if pressStrike {
			p.ShotType = ShotDrive
		} else {
			p.ShotType = ShotLob
		}
	}

	if in.Buttons&(BtnStrike|BtnLob) != 0 && int(p.ChargeTicks) < cfg.chargeMaxTicks {
		p.ChargeTicks++
	}

	if released(p, in, BtnStrike) || released(p, in, BtnLob) {
		freezeBuffer(p, in, cfg)
	}

	p.Held = in.Buttons
	return ev
}

// freezeBuffer latches a strike intention at release: the buffer window
// (bufTicks), the normalized charge, the requested world-space target and
// the requested spin level are all frozen so later ticks (and the eventual
// strike) see a stable snapshot even if the player's input keeps changing.
func freezeBuffer(p *Player, in PlayerInput, cfg *Config) {
	buf := cfg.bufTicks
	if buf < 1 {
		buf = 1 // a buffer of 0 ticks could never resolve a strike
	}
	if buf > math.MaxUint8 {
		buf = math.MaxUint8
	}
	p.Buffer = uint8(buf)
	if cfg.chargeMaxTicks > 0 {
		p.BufCharge = float64(p.ChargeTicks) / float64(cfg.chargeMaxTicks)
	}
	p.BufTarget = Vec3{X: in.TargetX, Y: BallRadius, Z: in.TargetZ}
	p.BufSpin = in.Spin
	p.ChargeTicks = 0
}

// improvesNextTick is the §7.1 "best tick of the window" heuristic: it
// compares the current error u against the error the same contact attempt
// would have one tick from now (ball advanced with the pure, event-free
// PredictBall; player advanced with its own Vel), and reports true when
// waiting one more tick would still improve things.
func improvesNextTick(s *State, p *Player, u float64, bicycleOK, serveOnly bool) bool {
	nextBall := PredictBall(s.Ball, &s.Cfg, 1)
	nextPos := p.Pos.AddScaled(p.Vel, Dt)

	hRel := nextBall.Pos.Y - nextPos.Y
	dH := math.Hypot(nextBall.Pos.X-nextPos.X, nextBall.Pos.Z-nextPos.Z)

	_, nextU := bestPartConstrained(&s.Cfg, hRel, dH, bicycleOK, serveOnly)
	return nextU < u
}

// pickStriker resolves at most one winner across every player with an ok
// candidate this tick (docs/design/contact-v2.md §7.3), -1 if nobody does.
func pickStriker(s *State, cand *[MaxPlayers]candidate) int8 {
	best := int8(-1)
	for i := int8(0); i < s.NumPlayers; i++ {
		if !cand[i].ok {
			continue
		}
		if best < 0 || strikerBeats(s, i, best, cand) {
			best = i
		}
	}
	return best
}

// strikerBeats reports whether candidate i should win over the current best
// (docs/design/contact-v2.md §7.3): lower error wins; ties go to the team
// that did not touch last (or, before any touch this rally, the server);
// a remaining tie (teammates in 2v2) falls back to the lower player index,
// which is not in the doc but is required to stay deterministic and
// independent of iteration order.
func strikerBeats(s *State, i, best int8, cand *[MaxPlayers]candidate) bool {
	ci, cb := cand[i], cand[best]
	if ci.u != cb.u {
		return ci.u < cb.u
	}

	teamI, teamB := s.Players[i].Team, s.Players[best].Team
	if teamI != teamB {
		if s.Rally.LastTeam < 0 {
			return teamI == s.Server
		}
		iIsLast := teamI == s.Rally.LastTeam
		bIsLast := teamB == s.Rally.LastTeam
		if iIsLast != bIsLast {
			return !iIsLast
		}
	}
	return i < best
}

// strike resolves player i's winning contact into a ball velocity/spin
// (docs/design/contact-v2.md §7.2 strike()): quality from u, deterministic
// deviation of target/power, the analytic launch solver, momentum, and the
// final speed cap (an over-powered request simply falls short instead of
// exceeding the part's physical limit).
func strike(s *State, i int8, part BodyPart, u float64, ev []Event) []Event {
	p := &s.Players[i]
	cfg := &s.Cfg
	ps := &cfg.Parts[part]

	q := qualityOf(cfg, u)
	h := strikeHash(s.Tick, p.Team, s.Ball.Pos)

	shot := p.ShotType
	if shot == ShotLob && !ps.AllowLob {
		shot = ShotDrive
	}

	tgt := deviateTarget(cfg, p.BufTarget, s.Ball.Pos, ps, u, h)
	speed := shotSpeed(ps, p.BufCharge, u, h, cfg)
	speed += momentumBonus(p.Vel, s.Ball.Pos, tgt, cfg)

	v := solveLaunch(cfg, s.Ball.Pos, tgt, shot, speed, p.BufCharge)
	maxSpeed := ps.SpeedMax + cfg.MomentumMax
	s.Ball.Vel = liftOverNet(cfg, s.Ball.Pos, tgt, clampLen(v, maxSpeed), maxSpeed)
	s.Ball.Spin = clamp(float64(p.BufSpin), ps.SpinMin, ps.SpinMax)
	s.Ball.Resting = false
	s.Ball.Out = false
	p.StruckAt = s.Tick

	if s.Phase == PhaseServe {
		s.Phase = PhaseRally
	}
	return append(ev, kickEvent(s, i, part, q))
}

// kickEvent builds the EvKick carrying everything the network/feedback
// contract needs (docs/design/contact-v2.md §11.1): the body part used, the
// resolved quality, and the ball's post-contact velocity/spin.
func kickEvent(s *State, i int8, part BodyPart, q Quality) Event {
	p := &s.Players[i]
	return Event{
		Kind: EvKick, Tick: s.Tick, Pos: s.Ball.Pos, Side: p.Team, Player: i,
		Part: part, Quality: q, Vel: s.Ball.Vel, Spin: s.Ball.Spin,
	}
}

// whiffEvent reports that player i's strike buffer expired with no contact.
func whiffEvent(s *State, i int8) Event {
	p := &s.Players[i]
	return Event{Kind: EvWhiff, Tick: s.Tick, Pos: s.Ball.Pos, Side: p.Team, Player: i}
}
