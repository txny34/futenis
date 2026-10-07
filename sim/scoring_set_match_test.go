package sim

import "testing"

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
