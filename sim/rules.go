package sim

// PointPauseTicks is how many ticks PhasePoint lasts before the next serve
// is set up (1.5 seconds at TickRate).
const PointPauseTicks = TickRate * 3 / 2

// serveBallOffset is how far from the net (on the server's side) the ball is
// placed for a new serve, so it starts within the server's KickReach.
const serveBallOffset = 6.5

// serveBallHeight is the ball's starting Y position for a new serve.
const serveBallHeight = 0.5

// servePlayerOffset is how far from the net each player starts for a new
// serve. wave1-C: must clear the server's own ServiceLineDist (6.95m) by
// more than ServeTossOffset (0.5m) toward the net, or the mandatory toss
// bounce (docs/design/contact-v2.md §7.5) would be geometrically
// impossible from the default serve position.
const servePlayerOffset = 8.0

// servePlayerZOffset is how far right/left of the center line the two
// partners of a 2v2 team start for a new serve. Wave 0 placeholder: real
// padel doubles starting positions / serve-box geometry is a wave1 concern
// (docs/design/contact-v2.md §7.5, "Modos y 2v2").
const servePlayerZOffset = CourtHalfWidth / 2

// applyRules reads the events emitted this tick, updates RallyState and
// Score, and appends EvPoint / EvGameOver when a point is decided. Events
// are only judged while Phase == PhaseRally; during PhasePoint the ball may
// still be flying but no event changes the score. ModePlayground never
// scores: it runs the ball-machine judge instead (applyPlayground,
// docs/design/training.md).
func applyRules(s *State, ev []Event) []Event {
	if s.Cfg.Mode == ModePlayground {
		return applyPlayground(s, ev)
	}
	switch s.Phase {
	case PhaseRally:
		return judgeRally(s, ev)
	case PhaseServe:
		// wave1-C: real-padel serve (docs/design/contact-v2.md §7.5) --
		// judges the pre-strike toss bounce; see serve.go.
		ev = judgeServe(s, ev)
		return enforceServeClock(s, ev)
	case PhasePoint:
		s.Rally.PauseTicks--
		if s.Rally.PauseTicks <= 0 {
			resetForServe(s, s.Server)
		}
	}
	return ev
}

// enforceServeClock is the §7.5 padel-like serve clock (code-review
// hardening, Config.ServeTimeLimit): if the server sits in PhaseServe
// without releasing the ball for ServeTimeLimit seconds, the ball is
// auto-dropped (tossServe) so an idle/disconnected/AI-less server can never
// hang the match forever. Runs after judgeServe, so a fault judged this
// same tick (which replays into a fresh RallyState, ServeWaitTicks back to
// 0) is never double-counted, and a legal player-initiated toss this same
// tick (Ball.Tossed already true) resets the clock instead of firing again.
func enforceServeClock(s *State, ev []Event) []Event {
	if s.Phase != PhaseServe || s.Ball.Tossed {
		s.Rally.ServeWaitTicks = 0
		return ev
	}
	s.Rally.ServeWaitTicks++
	if s.Rally.ServeWaitTicks < s.Cfg.serveTimeLimitTicks {
		return ev
	}
	return append(ev, tossServe(s))
}

// judgeRally walks the events emitted this tick (in emission order) looking
// for the first one that decides the point, and awards it if found. Events
// appended by awardPoint itself (EvPoint/EvGameOver) are not re-judged.
func judgeRally(s *State, ev []Event) []Event {
	n := len(ev)
	for i := 0; i < n; i++ {
		e := ev[i]
		// wave1-C: a touch before the serve's own mandatory toss bounce
		// (docs/design/contact-v2.md §7.5: "golpearla sin que haya picado
		// ... es falta") is never a legal rally start, even though
		// strike.go already flipped Phase to PhaseRally for it.
		if s.Rally.LastTeam < 0 && !s.Rally.ServeTossBounced && (e.Kind == EvKick || e.Kind == EvBodyHit) {
			// serveFault always leaves PhaseRally (either a retry resets to
			// PhaseServe, or a double fault awards the point), so there is
			// never anything left in this tick worth re-judging.
			return serveFault(s, e.Side, ev)
		}
		// wave1-C: while RallyState.ServeBoxPending, this crossing is still
		// the serve's own return flight -- it needs the real-padel box/let/
		// fault judge (serve.go) instead of the generic one-touch rules.
		if s.Rally.ServeBoxPending {
			handled, nev := judgeServeReturn(s, e, ev)
			ev = nev
			if handled {
				if s.Phase != PhaseRally {
					return ev // a fault/let/point already left PhaseRally
				}
				continue
			}
		}
		winner, decided := judgeEvent(&s.Rally, e)
		if decided {
			return awardPoint(s, winner, ev)
		}
	}
	return ev
}

// judgeEvent inspects a single tick event against the current RallyState and
// reports the winning team and whether the point is decided. It also
// advances RallyState.LastTeam/LastPlayer/BouncesOnSide for non-decisive
// kicks and bounces, enforcing the "max 1 touch per TEAM" padel rule
// (either partner may hit in 2v2, but only one touch total counts).
func judgeEvent(r *RallyState, e Event) (winner int8, decided bool) {
	k := r.LastTeam
	if k < 0 {
		return judgeFirstKick(r, e)
	}
	switch e.Kind {
	case EvKick, EvBodyHit:
		// wave1-C: EvBodyHit counts as a touch exactly like EvKick (docs/
		// design/contact-v2.md §7.4): bouncing off your own capsule after
		// the grace period is a double touch, same as kicking it twice.
		return judgeKickEvent(r, k, e)
	case EvBounceFloor:
		return judgeBounceEvent(r, k, e)
	case EvHitWall:
		if e.Side != k && r.BouncesOnSide[e.Side] == 0 {
			return 1 - k, true
		}
		return 0, false
	case EvOut:
		return faultOnKickerSide(r, k)
	default:
		return 0, false
	}
}

// judgeFirstKick handles the serve's opening kick, when RallyState has no
// LastTeam yet.
func judgeFirstKick(r *RallyState, e Event) (int8, bool) {
	if e.Kind == EvKick || e.Kind == EvBodyHit {
		r.LastTeam = e.Side
		r.LastPlayer = e.Player
		r.BouncesOnSide = [2]int{}
		r.ShotCount++
		markBodyGraze(r, e)
		// wave1-C: LastTeam is only ever -1 right after resetForServe/
		// replayServe, so this first touch is always the serve's opening
		// strike -- open the serve-return box judge (docs/design/
		// contact-v2.md §7.5; see serve.go).
		r.ServeBoxPending = true
	}
	return 0, false
}

// judgeKickEvent handles an EvKick once a rally is underway: a kick by the
// current kicker's own team is a double touch (faultOnKickerSide); a kick
// by the other team is a legal return that becomes the new LastTeam. A
// legal return also clears ServeUntouched (scoring.go's Ace detection):
// from here on the receiving team has touched the serve at least once.
func judgeKickEvent(r *RallyState, k int8, e Event) (int8, bool) {
	if e.Side == k {
		if mergesWithBodyGraze(r, e) {
			r.BodyGraze = false // the strike takes over the graze: still one touch
			return 0, false
		}
		return faultOnKickerSide(r, k)
	}
	r.LastTeam = e.Side
	r.LastPlayer = e.Player
	r.BouncesOnSide = [2]int{}
	r.ShotCount++
	r.ServeUntouched = false
	markBodyGraze(r, e)
	return 0, false
}

// bodyMergeTicks is how long after a body graze the same player's strike
// still counts as that same touch (0.1 s): the capsule meets the ball a few
// ticks before the foot does, which the player sees as a single touch.
const bodyMergeTicks = TickRate / 10

// mergesWithBodyGraze reports whether strike e is the same player finishing
// the touch their body just started (graze then strike inside the window).
func mergesWithBodyGraze(r *RallyState, e Event) bool {
	return e.Kind == EvKick && r.BodyGraze && e.Player == r.LastPlayer && e.Tick-r.BodyGrazeTick <= bodyMergeTicks
}

// markBodyGraze records whether the touch that just became LastTeam's was
// a body graze (EvBodyHit), for mergesWithBodyGraze.
func markBodyGraze(r *RallyState, e Event) {
	r.BodyGraze = e.Kind == EvBodyHit
	r.BodyGrazeTick = e.Tick
}

// judgeBounceEvent handles an EvBounceFloor: a bounce on the kicker's own
// side is a fault (faultOnKickerSide); a bounce on the receiving side counts
// toward the 1-bounce padel limit, awarding the kicker the point on the
// second bounce.
func judgeBounceEvent(r *RallyState, k int8, e Event) (int8, bool) {
	if e.Side == k {
		return faultOnKickerSide(r, k)
	}
	r.BouncesOnSide[e.Side]++
	if r.BouncesOnSide[e.Side] >= 2 {
		return k, true
	}
	return 0, false
}

// faultOnKickerSide resolves any fault that happens on the current kicker's
// side (double touch, own-side bounce, ball going out). If the ball already
// bounced legally on the receiving side, the receiver failed to return it in
// time and the kicker wins; otherwise the kicker's own shot was bad and the
// receiver wins.
func faultOnKickerSide(r *RallyState, k int8) (int8, bool) {
	if r.BouncesOnSide[1-k] > 0 {
		return k, true
	}
	return 1 - k, true
}

// deuceCap is how many points past Cfg.PointsToWin a deuce can run before
// the match is decided outright regardless of the 2-point margin
// (wave1-C scoring decision, documented here since Config has no field for
// it: a padel/volleyball-style "win by 2" decided every game eventually,
// without an unbounded deuce loop to reason about or test).
const deuceCap = 2

// hasWonGame reports whether winnerScore has clinched the game against
// loserScore: either the classic "first to PointsToWin, win by 2" rule
// (deuce-style, like padel/volleyball), or the hard deuceCap that
// guarantees every match terminates even through an extended deuce.
func hasWonGame(winnerScore, loserScore, pointsToWin int) bool {
	if winnerScore < pointsToWin {
		return false
	}
	if winnerScore-loserScore >= 2 {
		return true
	}
	return winnerScore >= pointsToWin+deuceCap
}

// awardPoint dispatches to the configured scoring model (docs/design/
// contact-v2.md "Puntuación de pádel"): ScoringSimple keeps today's flat
// point-tally behaviour (awardPointSimple) untouched; ScoringPadel runs the
// real points/games/sets model (scoring.go's awardPointPadel).
func awardPoint(s *State, winner int8, ev []Event) []Event {
	if s.Cfg.ScoringMode == ScoringPadel {
		return awardPointPadel(s, winner, ev)
	}
	return awardPointSimple(s, winner, ev)
}

// awardPointSimple increments the winner's score, emits EvPoint, and either
// ends the match (EvGameOver, once hasWonGame agrees) or starts the point
// pause before the next serve. This is the legacy v1 "first to
// Config.PointsToWin, win by 2" model, kept byte-for-byte for
// ScoringSimple (the zero value -- every pre-existing test and
// Playground/tuning use).
func awardPointSimple(s *State, winner int8, ev []Event) []Event {
	s.Score[winner]++
	ev = append(ev, Event{Kind: EvPoint, Tick: s.Tick, Side: winner, Player: -1})
	if hasWonGame(s.Score[winner], s.Score[1-winner], s.Cfg.PointsToWin) {
		s.Phase = PhaseGameOver
		return append(ev, Event{Kind: EvGameOver, Tick: s.Tick, Side: winner, Player: -1})
	}
	s.Phase = PhasePoint
	s.Server = winner
	s.Rally.PauseTicks = PointPauseTicks
	return ev
}

// resetForServe places every active player and the ball for a new serve by
// `server` (a team), resetting RallyState so the ball is in the server's
// reach and the previous rally's bookkeeping cannot leak into the next
// point. Also picks which individual player serves (serverPlayerIndex,
// alternating partners in 2v2) and the diagonal serve box
// (serveBoxForScore) -- docs/design/contact-v2.md §7.5 / "Modos y 2v2".
func resetForServe(s *State, server int8) {
	s.Server = server
	s.Phase = PhaseServe

	sp := serverPlayerIndex(s, server)
	s.LastServerPlayer[server] = sp

	s.Rally = RallyState{
		LastTeam:     -1,
		LastPlayer:   -1,
		ServerPlayer: sp,
		ServeAttempt: 1,
		ServeBox:     serveBoxForScore(s.Score),
	}

	placeServe(s, server)
}

// setupDoublesPositions places all 4 players for a 2v2 serve: each team's
// two partners start on either side of the center line, Slot 0 on that
// team's OWN right (rightSign: +Z for team 0, -Z for team 1), Slot 1 on its
// left, per the teamPlayers index convention (team 0 = {0,1}, team 1 =
// {2,3}). placeServe then moves the server onto its serve side.
func setupDoublesPositions(s *State) {
	for i := int8(0); i < 4; i++ {
		team, slot := i/2, i%2
		z := rightSign(team) * servePlayerZOffset
		if slot == 1 {
			z = -z
		}
		x := -forwardSign(team) * servePlayerOffset
		s.Players[i] = Player{Pos: Vec3{X: x, Z: z}, Team: team, Slot: slot, OnGround: true, Stamina: 1}
	}
}

// setupPlayground places the lone human straight into PhaseRally with no
// serve/scoring state (Entrenamiento, docs/design/training.md §2): slot 1
// stays zero/inactive, and the first continuous feed is scheduled one
// FeedInterval in (on demand waits for Match.Feed).
func setupPlayground(s *State) {
	s.Phase = PhaseRally
	s.Rally = RallyState{LastTeam: -1, LastPlayer: -1}
	s.Players[0] = Player{Pos: Vec3{X: -servePlayerOffset}, Team: 0, OnGround: true, Stamina: 1}
	s.Ball = Ball{Pos: Vec3{Y: serveBallHeight}}
	s.Feed = FeedState{}
	if !s.Cfg.FeedOnDemand {
		s.Feed.NextTick = s.Tick + uint32(s.Cfg.feedIntervalTicks)
	}
}
