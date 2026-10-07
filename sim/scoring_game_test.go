package sim

import "testing"

// --- hasWonGamePadel / pointDisplayCode / tiebreak helpers ----------------

func TestHasWonGamePadel(t *testing.T) {
	cases := []struct {
		name        string
		w, l        int
		golden      bool
		wantDecided bool
	}{
		{"0-0 never wins", 0, 0, false, false},
		{"3-0 (40-0) not yet", 3, 0, false, false},
		{"4-0 wins outright", 4, 0, false, true},
		{"4-3 (ad) not enough margin without golden point", 4, 3, false, false},
		{"5-3 wins by 2", 5, 3, false, true},
		{"4-3 wins immediately with golden point", 4, 3, true, true},
		{"3-3 (deuce) never wins even with golden point (no point played yet)", 3, 3, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := hasWonGamePadel(c.w, c.l, gamePoints, c.golden); got != c.wantDecided {
				t.Errorf("hasWonGamePadel(%d,%d,golden=%v): got %v, want %v", c.w, c.l, c.golden, got, c.wantDecided)
			}
		})
	}
}

func TestPointDisplayCode(t *testing.T) {
	cases := []struct {
		name        string
		mine, other int
		golden      bool
		want        int
	}{
		{"0-0", 0, 0, false, 0},
		{"15-0", 1, 0, false, 1},
		{"30-15", 2, 1, false, 2},
		{"40-30", 3, 2, false, 3},
		{"40-40 deuce", 3, 3, false, 3},
		{"advantage", 4, 3, false, 4},
		{"golden point never shows advantage", 4, 3, true, 3},
		{"lingering finished game clamps to 40-40", 5, 3, false, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pointDisplayCode(c.mine, c.other, c.golden); got != c.want {
				t.Errorf("pointDisplayCode(%d,%d,golden=%v): got %d, want %d", c.mine, c.other, c.golden, got, c.want)
			}
		})
	}
}

func TestHasWonSetGamesAndTiebreakTrigger(t *testing.T) {
	if !hasWonSetGames(3, 1, 3) {
		t.Errorf("3-1 should win a 3-game set")
	}
	if hasWonSetGames(3, 2, 3) {
		t.Errorf("3-2 should NOT win outright (only a 1-game margin)")
	}
	if !setNeedsTiebreak(3, 3, 3) {
		t.Errorf("3-3 should trigger a tiebreak at GamesPerSet=3")
	}
	if setNeedsTiebreak(2, 2, 3) {
		t.Errorf("2-2 should not trigger a tiebreak yet")
	}
}

// --- awardPoint dispatch: ScoringSimple is untouched -----------------------

func TestAwardPoint_DefaultsToScoringSimple(t *testing.T) {
	s := newRulesTestState() // ScoringMode left at its zero value
	ev := awardPoint(&s, 0, nil)

	if s.Score[0] != 1 {
		t.Fatalf("Score[0]: got %d, want 1 (legacy flat tally)", s.Score[0])
	}
	if s.Games != [2]int{0, 0} || s.Sets != [2]int{0, 0} {
		t.Errorf("Games/Sets: got %v/%v, want untouched by ScoringSimple", s.Games, s.Sets)
	}
	for _, e := range ev {
		if e.Kind == EvGameWon || e.Kind == EvSetWon || e.Kind == EvMatchWon {
			t.Fatalf("got a padel-only event %v in ScoringSimple", e.Kind)
		}
	}
}

// --- ScoringPadel: point -> game -------------------------------------------

func TestAwardPointPadel_PointsAccumulateUntilGameWon(t *testing.T) {
	s := newPadelTestState()

	ev := awardPoint(&s, 0, nil) // 15-0
	if s.Score[0] != 1 {
		t.Fatalf("Score[0]: got %d, want 1", s.Score[0])
	}
	if s.Games[0] != 0 {
		t.Fatalf("Games[0]: got %d, want 0 (game not decided at 15-0)", s.Games[0])
	}
	for _, e := range ev {
		if e.Kind == EvGameWon {
			t.Fatalf("got EvGameWon too early: %+v", ev)
		}
	}

	for i := 0; i < 3; i++ { // 30-0, 40-0, game
		ev = awardPoint(&s, 0, nil)
	}

	if s.Games[0] != 1 {
		t.Fatalf("Games[0]: got %d, want 1 (won at 4-0)", s.Games[0])
	}
	if s.Score != [2]int{0, 0} {
		t.Fatalf("Score: got %v, want reset to 0-0 for the new game", s.Score)
	}
	foundGameWon := false
	for _, e := range ev {
		if e.Kind == EvGameWon {
			foundGameWon = true
			if e.Side != 0 {
				t.Errorf("EvGameWon.Side: got %d, want 0", e.Side)
			}
		}
	}
	if !foundGameWon {
		t.Fatalf("got %+v, want an EvGameWon", ev)
	}
}

// TestAwardPointPadel_ServerAlternatesEveryGame covers real padel's game-by-
// game service alternation: whoever wins the game, the OTHER team serves
// next (never "winner serves").
func TestAwardPointPadel_ServerAlternatesEveryGame(t *testing.T) {
	s := newPadelTestState()
	s.Server = 0

	for i := 0; i < 4; i++ {
		awardPoint(&s, 1, nil) // team 1 wins every point/game
	}

	if s.Games[1] != 1 {
		t.Fatalf("Games[1]: got %d, want 1", s.Games[1])
	}
	if s.Server != 1 {
		t.Fatalf("Server: got %d, want 1 (alternates by game, independent of who won it)", s.Server)
	}
}

// TestAwardPointPadel_DeuceRequiresAdvantage covers the no-golden-point
// default: 3-3 then a single point does not win, a 2-point margin does.
func TestAwardPointPadel_DeuceRequiresAdvantage(t *testing.T) {
	s := newPadelTestState()
	s.Score = [2]int{3, 3}

	ev := awardPoint(&s, 0, nil) // advantage, not won yet
	if s.Games[0] != 0 {
		t.Fatalf("Games[0]: got %d, want 0 (only advantage, not a 2-point margin)", s.Games[0])
	}

	awardPoint(&s, 0, ev) // now wins by 2
	if s.Games[0] != 1 {
		t.Fatalf("Games[0]: got %d, want 1 (won on the second straight point past deuce)", s.Games[0])
	}
}

// TestAwardPointPadel_GoldenPointSuddenDeathAtDeuce covers Config.GoldenPoint:
// at 40-40, the very next point wins, no advantage required.
func TestAwardPointPadel_GoldenPointSuddenDeathAtDeuce(t *testing.T) {
	s := newPadelTestState()
	s.Cfg.GoldenPoint = true
	s.Score = [2]int{3, 3}

	awardPoint(&s, 1, nil)

	if s.Games[1] != 1 {
		t.Fatalf("Games[1]: got %d, want 1 (golden point sudden death)", s.Games[1])
	}
}
