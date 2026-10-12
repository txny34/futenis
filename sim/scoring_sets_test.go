package sim

import "testing"

// Simple scoring with sets (SetsToWin > 1, 1v1/2v2): each set is first to
// PointsToWin (win by 2, capped); winning a set emits EvSetWon and zeroes
// the score; the match ends at SetsToWin sets. SET POINT / MATCH POINT
// warnings tell which one the next point decides.

func setsState(score [2]int, sets [2]int) State {
	s := State{Cfg: DefaultConfig(), NumPlayers: 2}
	s.Cfg.Mode, s.Cfg.PointsToWin, s.Cfg.SetsToWin = Mode1v1, 5, 2
	s.Score, s.Sets = score, sets
	return s
}

func kinds(ev []Event) map[EventKind][]int8 {
	m := map[EventKind][]int8{}
	for _, e := range ev {
		m[e.Kind] = append(m[e.Kind], e.Side)
	}
	return m
}

func TestSets_WinningASetResetsTheScore(t *testing.T) {
	s := setsState([2]int{4, 2}, [2]int{0, 0})
	k := kinds(awardPointSimple(&s, 0, nil))
	if len(k[EvSetWon]) != 1 || k[EvSetWon][0] != 0 {
		t.Fatalf("events %v, want EvSetWon for team 0", k)
	}
	if len(k[EvGameOver]) != 0 || s.Phase == PhaseGameOver {
		t.Fatal("one set of two is not the match")
	}
	if s.Sets != [2]int{1, 0} || s.Score != [2]int{0, 0} {
		t.Fatalf("sets=%v score=%v, want [1 0] and a fresh [0 0]", s.Sets, s.Score)
	}
}

func TestSets_SecondSetWinsTheMatch(t *testing.T) {
	s := setsState([2]int{2, 4}, [2]int{1, 1})
	k := kinds(awardPointSimple(&s, 1, nil))
	if len(k[EvGameOver]) != 1 || k[EvGameOver][0] != 1 || s.Phase != PhaseGameOver {
		t.Fatalf("events %v phase %d, want game over for team 1", k, s.Phase)
	}
	if s.Sets != [2]int{1, 2} {
		t.Fatalf("sets=%v, want [1 2] (the final score stays visible)", s.Sets)
	}
}

func TestSets_SetPointVersusMatchPoint(t *testing.T) {
	cases := []struct {
		name            string
		score, sets     [2]int
		winner          int8
		wantSet, wantMP []int8
	}{
		{"first set 3-2 -> 4-2: set point", [2]int{3, 2}, [2]int{0, 0}, 0, []int8{0}, nil},
		{"team 0 up a set, 3-2 -> 4-2: match point", [2]int{3, 2}, [2]int{1, 0}, 0, nil, []int8{0}},
		{"team 1 up a set, 3-3 -> 4-3 team 0: set point for 0", [2]int{3, 3}, [2]int{0, 1}, 0, []int8{0}, nil},
		{"one set all, 3-3 -> 3-4: match point for 1", [2]int{3, 3}, [2]int{1, 1}, 1, nil, []int8{1}},
		{"6-6 at the cap, team 0 up a set: MP 0, SP 1", [2]int{6, 5}, [2]int{1, 0}, 1, []int8{1}, []int8{0}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := setsState(c.score, c.sets)
			k := kinds(awardPointSimple(&s, c.winner, nil))
			if !sameSides(k[EvSetPoint], c.wantSet) || !sameSides(k[EvMatchPoint], c.wantMP) {
				t.Fatalf("set points %v match points %v, want %v / %v", k[EvSetPoint], k[EvMatchPoint], c.wantSet, c.wantMP)
			}
		})
	}
}

func TestSets_OneSetKeepsTheLegacyFlow(t *testing.T) {
	s := setsState([2]int{4, 1}, [2]int{0, 0})
	s.Cfg.SetsToWin = 1
	k := kinds(awardPointSimple(&s, 0, nil))
	if len(k[EvSetWon]) != 0 || len(k[EvGameOver]) != 1 || s.Score != [2]int{5, 1} {
		t.Fatalf("events %v score %v: SetsToWin 1 must end on the score, no set events", k, s.Score)
	}
}

func sameSides(got, want []int8) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
