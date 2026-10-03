package sim

import "testing"

// Regression (browser bug "one ball scores the whole set"): a padel point
// that does not finish the game must still end the rally (PhasePoint), or
// judgeRally keeps awarding points off the same live ball every tick.
func TestAwardPointPadel_NonGamePointEntersPointPause(t *testing.T) {
	s := newPadelTestState()
	s.Phase = PhaseRally
	awardPoint(&s, 1, nil)
	if s.Score != [2]int{0, 1} {
		t.Fatalf("score: got %v, want [0 1]", s.Score)
	}
	if s.Phase != PhasePoint || s.Rally.PauseTicks != PointPauseTicks {
		t.Fatalf("phase=%d pause=%d, want PhasePoint/%d", s.Phase, s.Rally.PauseTicks, PointPauseTicks)
	}
	ev := applyRules(&s, []Event{{Kind: EvBounceFloor, Side: 0}, {Kind: EvBounceFloor, Side: 0}})
	if s.Score != [2]int{0, 1} || len(ev) != 2 {
		t.Fatalf("ball after point must not score again: score %v events %d", s.Score, len(ev))
	}
}
