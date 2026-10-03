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
// serve.
const servePlayerOffset = 7.0

// servePlayerZOffset is how far right/left of the center line the two
// partners of a 2v2 team start for a new serve. Wave 0 placeholder: real
// padel doubles starting positions / serve-box geometry is a wave1 concern
// (docs/design/contact-v2.md §7.5, "Modos y 2v2").
const servePlayerZOffset = CourtHalfWidth / 2

// applyRules reads the events emitted this tick, updates RallyState and
// Score, and appends EvPoint / EvGameOver when a point is decided. Events
// are only judged while Phase == PhaseRally; during PhasePoint the ball may
// still be flying but no event changes the score. ModePlayground never
// scores (docs/design/contact-v2.md "Modos y 2v2"): the ball just keeps
// bouncing and RespawnBall is the only way to reset it.
func applyRules(s *State, ev []Event) []Event {
	if s.Cfg.Mode == ModePlayground {
		return ev
	}
	switch s.Phase {
	case PhaseRally:
		return judgeRally(s, ev)
	case PhasePoint:
		s.Rally.PauseTicks--
		if s.Rally.PauseTicks <= 0 {
			resetForServe(s, s.Server)
		}
	}
	return ev
}

// judgeRally walks the events emitted this tick (in emission order) looking
// for the first one that decides the point, and awards it if found. Events
// appended by awardPoint itself (EvPoint/EvGameOver) are not re-judged.
func judgeRally(s *State, ev []Event) []Event {
	n := len(ev)
	for i := 0; i < n; i++ {
		winner, decided := judgeEvent(&s.Rally, ev[i])
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
	case EvKick:
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
	if e.Kind == EvKick {
		r.LastTeam = e.Side
		r.LastPlayer = e.Player
		r.BouncesOnSide = [2]int{}
	}
	return 0, false
}

// judgeKickEvent handles an EvKick once a rally is underway: a kick by the
// current kicker's own team is a double touch (faultOnKickerSide); a kick
// by the other team is a legal return that becomes the new LastTeam.
func judgeKickEvent(r *RallyState, k int8, e Event) (int8, bool) {
	if e.Side == k {
		return faultOnKickerSide(r, k)
	}
	r.LastTeam = e.Side
	r.LastPlayer = e.Player
	r.BouncesOnSide = [2]int{}
	return 0, false
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

// awardPoint increments the winner's score, emits EvPoint, and either ends
// the match (EvGameOver) or starts the point pause before the next serve.
func awardPoint(s *State, winner int8, ev []Event) []Event {
	s.Score[winner]++
	ev = append(ev, Event{Kind: EvPoint, Tick: s.Tick, Side: winner, Player: -1})
	if s.Score[winner] >= s.Cfg.PointsToWin {
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

	if s.NumPlayers >= 4 {
		setupDoublesPositions(s)
	} else {
		s.Players[0] = Player{Pos: Vec3{X: -servePlayerOffset}, Team: 0, OnGround: true, Stamina: 1}
		s.Players[1] = Player{Pos: Vec3{X: servePlayerOffset}, Team: 1, OnGround: true, Stamina: 1}
	}

	ballX := -serveBallOffset
	if server == 1 {
		ballX = serveBallOffset
	}
	s.Ball = Ball{Pos: Vec3{X: ballX, Y: serveBallHeight}}
}

// setupDoublesPositions places all 4 players for a 2v2 serve: each team's
// two partners start on either side of the center line (Slot 0 = right, 1 =
// left), per the teamPlayers index convention (team 0 = {0,1}, team 1 =
// {2,3}).
func setupDoublesPositions(s *State) {
	s.Players[0] = Player{Pos: Vec3{X: -servePlayerOffset, Z: servePlayerZOffset}, Team: 0, Slot: 0, OnGround: true, Stamina: 1}
	s.Players[1] = Player{Pos: Vec3{X: -servePlayerOffset, Z: -servePlayerZOffset}, Team: 0, Slot: 1, OnGround: true, Stamina: 1}
	s.Players[2] = Player{Pos: Vec3{X: servePlayerOffset, Z: servePlayerZOffset}, Team: 1, Slot: 0, OnGround: true, Stamina: 1}
	s.Players[3] = Player{Pos: Vec3{X: servePlayerOffset, Z: -servePlayerZOffset}, Team: 1, Slot: 1, OnGround: true, Stamina: 1}
}

// setupPlayground places a minimal 1v1-shaped pair straight into PhaseRally
// with no serve/scoring state: ModePlayground is a free sandbox
// (docs/design/contact-v2.md "Modos y 2v2").
func setupPlayground(s *State) {
	s.Phase = PhaseRally
	s.Rally = RallyState{LastTeam: -1, LastPlayer: -1}
	s.Players[0] = Player{Pos: Vec3{X: -servePlayerOffset}, Team: 0, OnGround: true, Stamina: 1}
	s.Players[1] = Player{Pos: Vec3{X: servePlayerOffset}, Team: 1, OnGround: true, Stamina: 1}
	s.Ball = Ball{Pos: Vec3{Y: serveBallHeight}}
}
