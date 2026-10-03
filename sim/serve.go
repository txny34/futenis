package sim

import "math"

// This file is wave1-C's real-padel serve judge (docs/design/contact-v2.md
// §7.5, overriding the original self-toss design): the drop, the required
// toss bounce behind the service line, the diagonal service box, lets,
// faults and double faults. It only judges events and mutates State/Rally;
// it never reads PlayerInput -- the strike pipeline (wave1 strike.go) is
// expected to call tossServe on the server's BtnStrike press edge while
// Phase == PhaseServe && !Ball.Tossed (per the doc's updateCharge comment),
// and to keep emitting the usual EvKick/EvBodyHit/EvWhiff the judge below
// reacts to.

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

// serveBoxZRange returns the inclusive Z range for diagonal box 0 (right)
// or 1 (left). Box 0/1 are a fixed physical Z-half of the court (not
// relative to either player's exact standing position): real padel's
// "diagonal" geometry collapses to this because the server's own
// right/left half and the receiver's right/left half are the same
// physical half of the court, just crossed in X by the net.
func serveBoxZRange(box int8) (zMin, zMax float64) {
	if box == 0 {
		return 0, CourtHalfWidth
	}
	return -CourtHalfWidth, 0
}

// inServeBox reports whether pos is a legal landing spot for server's
// serve into diagonal box.
func inServeBox(pos Vec3, server, box int8, cfg *Config) bool {
	xMin, xMax := serveBoxXRange(server, cfg)
	zMin, zMax := serveBoxZRange(box)
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
	default:
		return false, ev
	}
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
	}
	placeServe(s, server)
}

// placeServe positions every active player and the ball for a serve by
// `server` (shared by resetForServe's fresh point and replayServe's
// same-point retry).
func placeServe(s *State, server int8) {
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
