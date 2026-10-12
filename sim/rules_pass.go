package sim

import "math"

// 2v2 passes (futvóley): a team may touch the ball up to maxDoublesTouches
// times per crossing, never the same player twice in a row, and the
// 1-bounce limit on its side holds for the whole possession (a pass does
// not grant a new bounce). 1v1 keeps padel's single touch. The serve itself
// cannot be passed: the server's partner touching it is a double touch.

// maxDoublesTouches is the per-team touch cap per crossing in 2v2.
const maxDoublesTouches = 3

// touchLimit is RallyState.TouchLimit for s's mode: 2v2 allows passes.
func touchLimit(s *State) uint8 {
	if s.NumPlayers >= 4 {
		return maxDoublesTouches
	}
	return 1
}

// limitOf reads TouchLimit, treating the zero value as padel's single touch.
func limitOf(r *RallyState) uint8 {
	if r.TouchLimit < 1 {
		return 1
	}
	return r.TouchLimit
}

// touchesOf is TeamTouches, at least 1 once a team is in possession (states
// built before the counter existed read 0).
func touchesOf(r *RallyState) uint8 {
	if r.TeamTouches < 1 {
		return 1
	}
	return r.TeamTouches
}

// isLegalPass reports whether e, a touch by the team already in possession,
// is a legal pass instead of a double touch.
func isLegalPass(r *RallyState, e Event) bool {
	return mayTouchAgain(r, e.Player, e.Pos.X)
}

// mayTouchAgain is the shared rule for another touch by the team in
// possession: not the serve's flight, not the last toucher, a touch left,
// and the ball has not reached the rival side (no rival bounce yet, still
// over the own half) -- after that the shot is the rival's to answer.
func mayTouchAgain(r *RallyState, player int8, ballX float64) bool {
	k := r.LastTeam
	return !r.ServeBoxPending && player != r.LastPlayer && touchesOf(r) < limitOf(r) &&
		r.BouncesOnSide[1-k] == 0 && onOwnHalf(k, ballX)
}

// onOwnHalf reports whether x is on team's half (team 0 plays from -X).
func onOwnHalf(team int8, x float64) bool {
	if team == 0 {
		return x <= 0
	}
	return x >= 0
}

// takePass records a legal pass: same possession, one more team touch. The
// bounce count on the team's side is kept (one bounce per possession).
func takePass(r *RallyState, e Event) {
	r.LastPlayer = e.Player
	r.TeamTouches = touchesOf(r) + 1
	markBodyGraze(r, e)
}

// ownBounceAllowed reports whether the ball may bounce on the side of the
// team in possession k: only before it ever reached the rival side, while
// the team still has a touch left, and only once per possession.
func ownBounceAllowed(r *RallyState, k int8) bool {
	return r.BouncesOnSide[1-k] == 0 && touchesOf(r) < limitOf(r) && r.BouncesOnSide[k] == 0
}

// CanTouch reports whether player i may legally play the ball now in a
// rally: the other team holds it, or i's own team does, i was not the last
// to touch it, it is not the serve's flight and a touch is left.
func CanTouch(s *State, i int8) bool {
	r := &s.Rally
	if r.LastTeam < 0 {
		return false
	}
	if r.LastTeam != s.Players[i].Team {
		return true
	}
	return mayTouchAgain(r, i, s.Ball.Pos.X)
}

// PassReady reports whether player i's next touch can be a pass to the
// partner (snapshot flag and the strike's Lob -> pass switch): 2v2, rally
// under way past the serve, and after this touch the team still has one
// left for the shot that crosses.
func PassReady(s *State, i int8) bool {
	r := &s.Rally
	if s.Phase != PhaseRally || r.ServeBoxPending || r.LastTeam < 0 || limitOf(r) < 2 {
		return false
	}
	p := &s.Players[i]
	if r.LastTeam != p.Team {
		return true // first touch of the possession: 2 more after it
	}
	return mayTouchAgain(r, i, s.Ball.Pos.X) && touchesOf(r)+1 < limitOf(r)
}

// PassMaxCharge splits Globo in 2v2: released below it (a quick tap) the
// touch is a pass to the partner; charged to it or beyond, a normal lob.
const PassMaxCharge = 0.3

// passOnRelease reports whether player i releasing Lob now would pass:
// PassReady and the charge still under PassMaxCharge.
func passOnRelease(s *State, i int8, charge float64) bool {
	return charge < PassMaxCharge && PassReady(s, i)
}

// passHeight is where the set comes down to: the partner's chest, the
// easiest height to strike it again.
const passHeight = 1.0

// passLead is how far in front of the partner (toward the net) the set
// comes down, in metres.
const passLead = 1.0

// passCharge picks the set's apex between LobApexMin/Max: the lowest, a soft
// "levantada" the partner can read, not a defensive lob.
const passCharge = 0

// passVelocity is the 2v2 set (Lob + PassReady): a soft lob that comes down
// on player i's partner. No aim deviation and no power: the pass is a
// cooperative touch; the quality of the finishing shot is what matters.
func passVelocity(s *State, i int8) Vec3 {
	mate := &s.Players[i^1] // teamPlayers: team 0 = {0,1}, team 1 = {2,3}
	// In front of the partner (toward the net): the set comes down where a
	// step forward meets it with the foot, not on the head or body.
	toNet := -math.Copysign(1, mate.Pos.X)
	x := mate.Pos.X + float64(toNet*passLead)
	if x*mate.Pos.X <= 0 {
		x = mate.Pos.X // never over the net
	}
	tgt := Vec3{X: x, Y: passHeight, Z: mate.Pos.Z}
	return solveLaunch(&s.Cfg, s.Ball.Pos, tgt, ShotLob, 0, passCharge)
}
