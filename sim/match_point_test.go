package sim

import "testing"

// EvMatchPoint (simple scoring, 1v1/2v2): after a point, each team that
// would win the match with the next one is announced. First to 5, win by 2,
// capped at 7 (hasWonGame / deuceCap).
func TestMatchPoint(t *testing.T) {
	cases := []struct {
		name      string
		before    [2]int
		winner    int8
		wantSides []int8
	}{
		{"3-2 -> 4-2: team 0 has match point", [2]int{3, 2}, 0, []int8{0}},
		{"3-3 -> 3-4: team 1 has match point", [2]int{3, 3}, 1, []int8{1}},
		{"2-1 -> 3-1: nobody yet", [2]int{2, 1}, 0, nil},
		{"4-3 -> 4-4: deuce, nobody (win by 2)", [2]int{4, 3}, 1, nil},
		{"4-4 -> 5-4: advantage, team 0", [2]int{4, 4}, 0, []int8{0}},
		{"5-5 -> 6-5: cap 7 -> team 0", [2]int{5, 5}, 0, []int8{0}},
		{"6-5 -> 6-6: at the cap both are a point away", [2]int{6, 5}, 1, []int8{0, 1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := State{Cfg: DefaultConfig(), NumPlayers: 2}
			s.Cfg.Mode, s.Cfg.PointsToWin = Mode1v1, 5
			s.Score = c.before
			var got []int8
			for _, e := range awardPointSimple(&s, c.winner, nil) {
				if e.Kind == EvMatchPoint {
					got = append(got, e.Side)
				}
			}
			if len(got) != len(c.wantSides) {
				t.Fatalf("match points %v, want %v", got, c.wantSides)
			}
			for i := range got {
				if got[i] != c.wantSides[i] {
					t.Fatalf("match points %v, want %v", got, c.wantSides)
				}
			}
		})
	}
}

func TestMatchPoint_NotOnTheWinningPointNorOutsideMatches(t *testing.T) {
	s := State{Cfg: DefaultConfig(), NumPlayers: 2}
	s.Cfg.Mode, s.Cfg.PointsToWin = Mode1v1, 5
	s.Score = [2]int{4, 2}
	for _, e := range awardPointSimple(&s, 0, nil) {
		if e.Kind == EvMatchPoint {
			t.Fatal("the winning point is game over, not match point")
		}
	}
	s = State{Cfg: DefaultConfig(), NumPlayers: 1}
	s.Cfg.Mode, s.Cfg.PointsToWin = ModePlayground, 5
	s.Score = [2]int{3, 0}
	for _, e := range awardPointSimple(&s, 0, nil) {
		if e.Kind == EvMatchPoint {
			t.Fatal("no match point outside 1v1/2v2")
		}
	}
}
