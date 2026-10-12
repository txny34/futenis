package sim

import "math"

// This file is the real-padel serve judge (docs/design/contact-v2.md
// §7.5, overriding the original self-toss design): the drop, the required
// toss bounce behind the service line, the diagonal service box, lets,
// faults and double faults. It only judges events and mutates State/Rally;
// it never reads PlayerInput -- the strike pipeline (strike.go) calls
// tossServe on the server's BtnStrike press edge while
// Phase == PhaseServe && !Ball.Tossed, and emits the EvKick/EvBodyHit/
// EvWhiff the judge below reacts to.

// --- service box / toss-bounce geometry -------------------------------

// serveBoxXRange returns the inclusive X range (on the RECEIVING side of
// the net) the serve's own bounce must land in: between the net and the
// receiver's service line, same distance (Config.ServiceLineDist) used for
// the server's own pre-strike toss bounce.
func serveBoxXRange(server int8, cfg *Config) (xMin, xMax float64) {
	if server == 0 {
		return 0, cfg.ServiceLineDist
	}
	return -cfg.ServiceLineDist, 0
}

// rightSign returns the Z sign of team's own right, seen from its own back
// wall facing the net: team 0 faces +X so its right is +Z; team 1 faces -X
// so its right is -Z (docs/design/contact-v2.md §7.5 side convention).
func rightSign(team int8) float64 {
	if team == 0 {
		return 1
	}
	return -1
}

// ServeBoxZSign returns the Z sign of the physical court half that
// server's serve must land in for RallyState.ServeBox box: box 0 is the
// RECEIVER's right, box 1 the receiver's left. Exported so the bot (ai/)
// can aim/stand without duplicating the side convention.
func ServeBoxZSign(server, box int8) float64 {
	z := rightSign(1 - server)
	if box != 0 {
		return -z
	}
	return z
}

// serveStandZ returns where the server stands in Z for box: its own right
// for box 0 (even point total), its own left for box 1 -- always the half
// diagonally opposite the target box, so the serve crosses the court.
func serveStandZ(server, box int8) float64 {
	return -ServeBoxZSign(server, box) * servePlayerZOffset
}

// serveBoxZRange returns the inclusive Z range of server's diagonal box.
func serveBoxZRange(server, box int8) (zMin, zMax float64) {
	if ServeBoxZSign(server, box) > 0 {
		return 0, CourtHalfWidth
	}
	return -CourtHalfWidth, 0
}

// inServeBox reports whether pos is a legal landing spot for server's
// serve into diagonal box.
func inServeBox(pos Vec3, server, box int8, cfg *Config) bool {
	xMin, xMax := serveBoxXRange(server, cfg)
	zMin, zMax := serveBoxZRange(server, box)
	return pos.X >= xMin && pos.X <= xMax && pos.Z >= zMin && pos.Z <= zMax
}

// inServiceLineBounceZone reports whether pos is a legal spot for the
// PRE-strike toss bounce: on the server's own side, strictly behind
// (farther from the net than) the service line.
func inServiceLineBounceZone(pos Vec3, server int8, cfg *Config) bool {
	if sideOf(pos.X) != server {
		return false
	}
	return math.Abs(pos.X) > cfg.ServiceLineDist
}

// newServiceTurn reports whether the point about to be served by team
// starts a new service turn (2v2 partner rotation, serverPlayerIndex):
// ScoringPadel -- the first point of a game, or in a tiebreak every point
// where the serve changes hands (total points 0, 1, 3, 5, ...);
// ScoringSimple (no games) -- whenever team regains the serve from the
// other team (Rally.ServerPlayer still names the previous point's server).
func newServiceTurn(s *State, team int8) bool {
	if s.Cfg.ScoringMode != ScoringPadel {
		return s.Players[s.Rally.ServerPlayer].Team != team
	}
	total := s.Score[0] + s.Score[1]
	if isTiebreak(s) {
		return total == 0 || total%2 == 1
	}
	return total == 0
}

// --- the drop (toss) ----------------------------------------------------

// tossServe drops the ball for the current server's attempt: released at
// waist height (Config.ServeTossHeight) in front of the server
// (Config.ServeTossOffset toward the net), with gravity -- plus any
// Config.ServeTossSpeed upward assist, default 0 -- taking over from here.
// Called by strike.go's updateCharge on the server's BtnStrike press edge
// while Phase == PhaseServe && !Ball.Tossed; it does not itself start
// Phase == PhaseRally, that still only happens on the eventual legal
// strike.
func tossServe(s *State) Event {
	sp := s.Rally.ServerPlayer
	srv := &s.Players[sp]
	fwd := forwardSign(srv.Team)
	pos := Vec3{
		X: srv.Pos.X + float64(fwd*s.Cfg.ServeTossOffset),
		Y: s.Cfg.ServeTossHeight,
		Z: srv.Pos.Z,
	}
	s.Ball = Ball{Pos: pos, Vel: Vec3{Y: s.Cfg.ServeTossSpeed}, Tossed: true}
	return Event{Kind: EvToss, Tick: s.Tick, Pos: pos, Side: srv.Team, Player: sp}
}

// --- pre-strike judge (Phase == PhaseServe) ------------------------------
//
// From here on this file only judges what happens to the ball once it is
// in flight (the drop above, and the box/let/fault state machine below).

// judgeServe judges the events emitted while Phase == PhaseServe: the
// window between a fresh serve setup (resetForServe/replayServe) and the
// server's opening strike. The dropped ball must bounce exactly once,
// behind the server's own service line, before being struck; any other
// outcome -- the wrong spot, a second bounce, a wall/net/out before a
// legal bounce, or a whiff -- is a serve fault (docs/design/
// contact-v2.md §7.5).
func judgeServe(s *State, ev []Event) []Event {
	n := len(ev)
	for i := 0; i < n && s.Phase == PhaseServe; i++ {
		switch ev[i].Kind {
		case EvBounceFloor:
			ev = judgeServeToss(s, ev[i], ev)
		case EvHitWall, EvHitNet, EvOut, EvWhiff, EvBodyHit:
			ev = serveFault(s, s.Server, ev)
		}
	}
	return ev
}

// judgeServeToss resolves one EvBounceFloor seen during the pre-strike
// window: the first one must land behind the service line on the server's
// own side; a second one (the server never struck it in time) is also a
// fault.
func judgeServeToss(s *State, e Event, ev []Event) []Event {
	r := &s.Rally
	if r.ServeTossBounced {
		return serveFault(s, s.Server, ev)
	}
	if !inServiceLineBounceZone(e.Pos, s.Server, &s.Cfg) {
		return serveFault(s, s.Server, ev)
	}
	r.ServeTossBounced = true
	return ev
}

// --- post-strike judge (Phase == PhaseRally, RallyState.ServeBoxPending) -

// judgeServeReturn judges one event of the serve's own return flight: the
// crossing that opened with the server's strike (which set
// RallyState.ServeBoxPending via judgeFirstKick) and must land legally in
// the receiver's diagonal box before the normal one-touch rally rules take
// back over. handled reports whether this function owns the event's
// outcome (true for every kind it recognizes, even when nothing is
// decided yet, e.g. recording a net touch) -- judgeRally must not also run
// the generic judgeEvent for a handled event.
func judgeServeReturn(s *State, e Event, ev []Event) (handled bool, out []Event) {
	r := &s.Rally
	k := r.LastTeam
	switch e.Kind {
	case EvHitNet:
		if e.Side == k {
			r.ServeNetTouch = true
		}
		return true, ev
	case EvBounceFloor:
		return true, judgeServeBounce(s, k, e, ev)
	case EvHitWall:
		if e.Side == k {
			return true, ev // own-side wall before the return bounce is harmless
		}
		return true, serveFault(s, k, ev)
	case EvOut:
		return true, serveFault(s, k, ev)
	case EvKick, EvBodyHit:
		return judgeServeVolley(s, k, e, ev)
	default:
		return false, ev
	}
}

// judgeServeVolley handles a touch while the serve's own return flight is
// still pending (RallyState.ServeBoxPending): real padel requires the serve
// to bounce once in the receiver's diagonal box before anyone may play it.
// The receiving team touching it early (a volley return) forfeits the
// point to the server's team OUTRIGHT -- this is the receiver's fault, not
// the server's, so it bypasses serveFault/replayServe entirely (no serve
// attempt is consumed, no double-fault bookkeeping). A touch by the
// server's own team (e.Side == k, e.g. an own body hit before the ball even
// lands) is left unhandled so the generic one-touch double-touch rule
// (judgeKickEvent) decides it instead -- EXCEPT a touch by the server's
// partner (2v2): real padel calls a serve that hits the server's own
// partner a serve fault (counts toward a double fault), not a lost point.
func judgeServeVolley(s *State, k int8, e Event, ev []Event) (handled bool, out []Event) {
	if e.Side == k {
		if e.Player != s.Rally.ServerPlayer {
			return true, serveFault(s, k, ev)
		}
		return false, ev
	}
	return true, awardPoint(s, k, ev)
}

// judgeServeBounce resolves the serve's own first bounce after the
// strike: landing in the correct diagonal box on the receiving side is
// legal (a Let if the net was touched first, otherwise the box judge
// closes and the ball counts as a normal receiver-side bounce); any other
// landing spot -- including the server's own side, a mishit -- is a
// fault.
func judgeServeBounce(s *State, k int8, e Event, ev []Event) []Event {
	r := &s.Rally
	if e.Side == k || !inServeBox(e.Pos, k, r.ServeBox, &s.Cfg) {
		return serveFault(s, k, ev)
	}
	if r.ServeNetTouch {
		return letServe(s, k, ev)
	}
	r.ServeBoxPending = false
	r.BouncesOnSide[e.Side]++
	// scoring.go's Ace detection: the serve just landed legally and the
	// receiving team has not touched it yet. judgeKickEvent clears this the
	// moment they do; if the point is decided before that (a second bounce
	// on the receiving side with nobody touching it), it is an Ace.
	r.ServeUntouched = true
	return ev
}

// --- fault / let / replay -----------------------------------------------

// serveFault appends EvServeFault and resolves its consequence: the first
// fault of a point just replays with a second serve (ServeAttempt 1 -> 2);
// a second consecutive fault is a double fault, awarding the point to the
// receiver outright.
func serveFault(s *State, server int8, ev []Event) []Event {
	r := &s.Rally
	ev = append(ev, Event{Kind: EvServeFault, Tick: s.Tick, Side: server, Player: r.ServerPlayer})
	if r.ServeAttempt >= 2 {
		return awardPoint(s, 1-server, ev)
	}
	replayServe(s, server, 2)
	return ev
}

// letServe appends EvLet and replays the exact same serve attempt (does
// NOT count as a fault): the ball clipped the net but still landed in the
// correct diagonal box.
func letServe(s *State, server int8, ev []Event) []Event {
	r := &s.Rally
	ev = append(ev, Event{Kind: EvLet, Tick: s.Tick, Side: server, Player: r.ServerPlayer})
	replayServe(s, server, r.ServeAttempt)
	return ev
}

// replayServe resets the ball/players/Phase for another serve attempt by
// the same server, without touching the score and without re-picking the
// server player or the diagonal box (real padel keeps both fixed across
// the two serves of one point; only serveBoxForScore/serverPlayerIndex, in
// resetForServe, change them, and only between points).
func replayServe(s *State, server int8, attempt uint8) {
	box := s.Rally.ServeBox
	sp := s.Rally.ServerPlayer
	s.Server = server
	s.Phase = PhaseServe
	s.Rally = RallyState{
		LastTeam:     -1,
		LastPlayer:   -1,
		ServerPlayer: sp,
		ServeAttempt: attempt,
		ServeBox:     box,
		TouchLimit:   touchLimit(s),
	}
	placeServe(s, server)
}

// placeServe positions every active player and the ball for a serve by
// `server` (shared by resetForServe's fresh point and replayServe's
// same-point retry).
// The server stands on the half diagonally opposite RallyState.ServeBox
// (serveStandZ); the receiver(s) stand per their slot (1v1: diagonal to
// the server).
func placeServe(s *State, server int8) {
	z := serveStandZ(server, s.Rally.ServeBox)
	if s.NumPlayers >= 4 {
		setupDoublesPositions(s)
		putServerOnSide(s, server, z)
	} else {
		s.Players[0] = Player{Pos: Vec3{X: -servePlayerOffset}, Team: 0, OnGround: true, Stamina: 1}
		s.Players[1] = Player{Pos: Vec3{X: servePlayerOffset}, Team: 1, OnGround: true, Stamina: 1}
		s.Players[server].Pos.Z = z
		s.Players[1-server].Pos.Z = -z
	}

	ballX := -serveBallOffset
	if server == 1 {
		ballX = serveBallOffset
	}
	s.Ball = Ball{Pos: Vec3{X: ballX, Y: serveBallHeight, Z: z}}
}

// putServerOnSide swaps the serving team's two partners' Z (2v2) when the
// server (RallyState.ServerPlayer) is not already standing at z.
func putServerOnSide(s *State, server int8, z float64) {
	sp := s.Rally.ServerPlayer
	if s.Players[sp].Pos.Z == z {
		return
	}
	a, b := teamPlayers(server)
	s.Players[a].Pos.Z, s.Players[b].Pos.Z = s.Players[b].Pos.Z, s.Players[a].Pos.Z
}
