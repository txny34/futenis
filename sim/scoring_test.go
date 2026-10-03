package sim

import "testing"

// newPadelTestState builds a fresh ScoringPadel rules-test state, mirroring
// newRulesTestState (rules_test.go) but opted into the real scoring model.
func newPadelTestState() State {
	s := newRulesTestState()
	s.Cfg.ScoringMode = ScoringPadel
	s.Cfg.GamesPerSet = 3
	s.Cfg.SetsToWin = 1
	return s
}

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
			if got := hasWonGamePadel(c.w, c.l, c.golden); got != c.wantDecided {
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

	ev = awardPoint(&s, 0, ev) // now wins by 2
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

// --- EvGamePoint / EvBreakPoint --------------------------------------------

func TestAwardPointPadel_GamePointForServer(t *testing.T) {
	s := newPadelTestState()
	s.Server = 0
	s.Score = [2]int{2, 0} // 30-0

	ev := awardPoint(&s, 0, nil) // 40-0: server one point from winning

	found := false
	for _, e := range ev {
		if e.Kind == EvGamePoint {
			found = true
			if e.Side != 0 {
				t.Errorf("EvGamePoint.Side: got %d, want 0 (the server)", e.Side)
			}
		}
		if e.Kind == EvBreakPoint {
			t.Errorf("got an EvBreakPoint, want EvGamePoint (team 0 is serving)")
		}
	}
	if !found {
		t.Fatalf("got %+v, want an EvGamePoint", ev)
	}
}

func TestAwardPointPadel_BreakPointForReceiver(t *testing.T) {
	s := newPadelTestState()
	s.Server = 0
	s.Score = [2]int{0, 2} // receiver (team 1) at 30-0

	ev := awardPoint(&s, 1, nil) // 40-0 for the receiver: break point

	found := false
	for _, e := range ev {
		if e.Kind == EvBreakPoint {
			found = true
			if e.Side != 1 {
				t.Errorf("EvBreakPoint.Side: got %d, want 1 (the non-server)", e.Side)
			}
		}
		if e.Kind == EvGamePoint {
			t.Errorf("got an EvGamePoint, want EvBreakPoint (team 1 is not serving)")
		}
	}
	if !found {
		t.Fatalf("got %+v, want an EvBreakPoint", ev)
	}
}

// --- game -> set -> match ----------------------------------------------

func TestAwardPointPadel_TiebreakTriggeredAtGamesAllThenWinsSet(t *testing.T) {
	s := newPadelTestState()
	s.Games = [2]int{2, 2} // one game each short of GamesPerSet(3)... actually tie at 2-2, next game ties at 3-3
	s.Score = [2]int{3, 0}

	ev := awardPoint(&s, 0, nil) // wins the game: 3-2
	if s.Games[0] != 3 {
		t.Fatalf("Games[0]: got %d, want 3", s.Games[0])
	}
	if s.Sets != [2]int{0, 0} {
		t.Fatalf("Sets: got %v, want still 0-0 (3-2 is not a 2-game margin)", s.Sets)
	}
	for _, e := range ev {
		if e.Kind == EvSetWon {
			t.Fatalf("got an EvSetWon too early: %+v", ev)
		}
	}

	// Now even it up at 3-3 to trigger a tiebreak, then play the tiebreak
	// out: whoever wins THIS game (however many points it takes) wins the
	// set outright, regardless of the games margin.
	s.Games = [2]int{3, 3}
	s.Score = [2]int{0, 0}
	for i := 0; i < tiebreakTarget; i++ {
		ev = awardPoint(&s, 1, ev)
	}

	if s.Sets[1] != 1 {
		t.Fatalf("Sets[1]: got %d, want 1 (tiebreak winner takes the set)", s.Sets[1])
	}
	if s.Games != [2]int{0, 0} {
		t.Fatalf("Games: got %v, want reset to 0-0 for the new set", s.Games)
	}
	foundSetWon := false
	for _, e := range ev {
		if e.Kind == EvSetWon && e.Side == 1 {
			foundSetWon = true
		}
	}
	if !foundSetWon {
		t.Fatalf("got %+v, want an EvSetWon(side=1)", ev)
	}
}

func TestAwardPointPadel_MatchWonAtSetsToWin(t *testing.T) {
	s := newPadelTestState()
	s.Cfg.SetsToWin = 1
	s.Games = [2]int{2, 0}
	s.Score = [2]int{3, 0}

	ev := awardPoint(&s, 0, nil) // wins the game at 3-0 games -> wins the set -> wins the match

	if s.Phase != PhaseGameOver {
		t.Fatalf("Phase: got %v, want PhaseGameOver", s.Phase)
	}
	if s.Sets[0] != 1 {
		t.Fatalf("Sets[0]: got %d, want 1", s.Sets[0])
	}
	foundMatchWon := false
	for _, e := range ev {
		if e.Kind == EvMatchWon {
			foundMatchWon = true
			if e.Side != 0 {
				t.Errorf("EvMatchWon.Side: got %d, want 0", e.Side)
			}
		}
		if e.Kind == EvGameOver {
			t.Errorf("got the legacy EvGameOver in ScoringPadel, want only EvMatchWon")
		}
	}
	if !foundMatchWon {
		t.Fatalf("got %+v, want an EvMatchWon", ev)
	}
}

// --- Ace --------------------------------------------------------------------

// TestAwardPointPadel_AceWhenServeNeverTouched exercises the full judgeRally
// path: a legal serve bounce followed immediately by a second bounce on the
// receiving side (the receiver never touched it) must award the point to
// the server's team and emit EvAce.
func TestAwardPointPadel_AceWhenServeNeverTouched(t *testing.T) {
	s := newPadelTestState()
	s.Server = 0
	resetForServe(&s, 0)
	s.Rally.ServeTossBounced = true
	s.Phase = PhaseRally

	ev := judgeRally(&s, []Event{
		{Kind: EvKick, Side: 0, Player: 0},                    // server's opening strike
		{Kind: EvBounceFloor, Side: 1, Pos: Vec3{X: 3, Z: 1}}, // legal landing in the box
	})
	// First bounce only legalizes the box; nothing decided yet.
	for _, e := range ev {
		if e.Kind == EvAce || e.Kind == EvPoint {
			t.Fatalf("got %v too early (only the serve's own legal bounce happened): %+v", e.Kind, ev)
		}
	}

	ev = judgeRally(&s, []Event{
		{Kind: EvBounceFloor, Side: 1, Pos: Vec3{X: 3, Z: 1}}, // second bounce, still untouched
	})

	foundAce, foundPoint := false, false
	for _, e := range ev {
		if e.Kind == EvAce {
			foundAce = true
			if e.Side != 0 {
				t.Errorf("EvAce.Side: got %d, want 0 (the server)", e.Side)
			}
		}
		if e.Kind == EvPoint && e.Side == 0 {
			foundPoint = true
		}
	}
	if !foundAce || !foundPoint {
		t.Fatalf("got %+v, want both EvAce and EvPoint(side=0)", ev)
	}
}

// TestAwardPointPadel_NoAceWhenReceiverTouchesFirst covers the negative
// case: once the receiver legally returns the ball, a later point for the
// server's team must NOT be reported as an Ace.
func TestAwardPointPadel_NoAceWhenReceiverTouchesFirst(t *testing.T) {
	s := newPadelTestState()
	s.Server = 0
	resetForServe(&s, 0)
	s.Rally.ServeTossBounced = true
	s.Phase = PhaseRally

	judgeRally(&s, []Event{
		{Kind: EvKick, Side: 0, Player: 0},
		{Kind: EvBounceFloor, Side: 1, Pos: Vec3{X: 3, Z: 1}},
	})
	if !s.Rally.ServeUntouched {
		t.Fatalf("ServeUntouched: got false, want true right after the legal serve bounce")
	}

	judgeRally(&s, []Event{{Kind: EvKick, Side: 1, Player: 1}}) // receiver returns it
	if s.Rally.ServeUntouched {
		t.Fatalf("ServeUntouched: got true, want false once the receiver has touched it")
	}

	ev := judgeRally(&s, []Event{{Kind: EvOut, Side: 1}}) // now the server's shot goes out: server wins, but not an Ace
	for _, e := range ev {
		if e.Kind == EvAce {
			t.Fatalf("got an EvAce after the receiver already touched the ball: %+v", ev)
		}
	}
}

// --- RallyState.ShotCount ---------------------------------------------------

func TestRally_ShotCountIncrementsPerLegalTouch(t *testing.T) {
	r := RallyState{LastTeam: -1, LastPlayer: -1}
	judgeEvent(&r, Event{Kind: EvKick, Side: 0, Player: 0})
	if r.ShotCount != 1 {
		t.Fatalf("ShotCount after serve: got %d, want 1", r.ShotCount)
	}
	judgeEvent(&r, Event{Kind: EvKick, Side: 1, Player: 1})
	if r.ShotCount != 2 {
		t.Fatalf("ShotCount after the return: got %d, want 2", r.ShotCount)
	}
}
