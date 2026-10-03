package sim

import "testing"

// TestRally_ShotAnywhereInRivalCourtWinsWhenNotReturned is the padel rule
// the player expects: once the serve is over, a shot (volley or not) whose
// first contact is ANY spot of the rival floor is legal; if the rival does
// not return it, the striker wins the point. Checked end to end through
// strike (aim assist) + physics + rules, for every timing quality.
func TestRally_ShotAnywhereInRivalCourtWinsWhenNotReturned(t *testing.T) {
	parts := []BodyPart{PartFoot, PartKnee, PartChest, PartHead}
	for _, part := range parts {
		for _, u := range []float64{0, 0.5, 1} {
			for _, tx := range []float64{1, 3, 5, 7, 9.5} {
				for _, tz := range []float64{-4.5, 0, 4.5} {
					for _, px := range []float64{-9, -5, -1.5} {
						w, first := rallyShotTo(part, px, tx, tz, u)
						if first != EvBounceFloor || w != 0 {
							t.Errorf("part=%d u=%.1f from x=%.1f to (%.1f,%.1f): first rival contact=%d (255=none) winner=%d, want floor + team 0",
								part, u, px, tx, tz, first, w)
						}
					}
				}
			}
		}
	}
}

// rallyShotTo sets up a live rally (serve already returned, box judge
// closed), has player 0 strike a ball at the part's ideal height toward
// (tx, tz) while the rival stands still in a far corner, then steps the full
// Match (physics + rules) until a point is decided. It returns the winner
// (-1 if none) and the kind of the ball's first contact on the rival side.
func rallyShotTo(part BodyPart, px, tx, tz, u float64) (int8, EventKind) {
	cfg := DefaultConfig()
	cfg.ScoringMode = ScoringPadel
	m := NewMatch(cfg)
	s := m.state
	s.Tick = 500 // StruckAt == 0 means "never struck": a real strike never happens on tick 0
	s.Phase = PhaseRally
	s.Rally = RallyState{LastTeam: 1, LastPlayer: 1}
	s.Players[0].Pos = Vec3{X: px}
	s.Players[0].BufTarget = Vec3{X: tx, Y: BallRadius, Z: tz}
	s.Players[0].BufCharge = 0.5
	s.Players[1].Pos = Vec3{X: 9.5, Z: 4.5}
	s.Ball = Ball{Pos: Vec3{X: px, Y: s.Cfg.Parts[part].HIdeal}}
	strike(&s, 0, part, u, nil)
	s.Rally.LastTeam, s.Rally.LastPlayer = 0, 0
	m.Restore(s)

	const noEvent = EventKind(255)
	first := noEvent
	for i := 0; i < TickRate*6; i++ {
		for _, e := range m.Step([MaxPlayers]PlayerInput{}) {
			if first == noEvent && e.Side == 1 && (e.Kind == EvBounceFloor || e.Kind == EvHitWall || e.Kind == EvOut) {
				first = e.Kind
			}
			if e.Kind == EvPoint {
				return e.Side, first
			}
		}
	}
	return -1, first
}
