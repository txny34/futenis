package sim

import "testing"

// --- EvBodyHit folded into the generic one-touch-per-team judge ------------

// TestJudgeEvent_BodyHit mirrors TestJudgeEvent (rules_test.go) but for
// EvBodyHit, covering the doc's "EvBodyHit counts as a touch exactly like
// EvKick" rule (docs/design/contact-v2.md §7.4) including the 2v2 case
// where a player's OWN partner body-hits the ball.
func TestJudgeEvent_BodyHit(t *testing.T) {
	cases := []struct {
		name        string
		rally       RallyState
		event       Event
		wantWinner  int8
		wantDecided bool
	}{
		{
			name:  "first touch of the rally may be a body hit",
			rally: RallyState{LastTeam: -1},
			event: Event{Kind: EvBodyHit, Side: 0, Player: 0},
		},
		{
			name:  "legal return via body hit by the other team",
			rally: RallyState{LastTeam: 0},
			event: Event{Kind: EvBodyHit, Side: 1, Player: 1},
		},
		{
			name:        "body hit by the same team that last touched is a double touch",
			rally:       RallyState{LastTeam: 0, LastPlayer: 0, BouncesOnSide: [2]int{0, 0}},
			event:       Event{Kind: EvBodyHit, Side: 0, Player: 0},
			wantWinner:  1,
			wantDecided: true,
		},
		{
			name:        "a body hit by the kicker's OWN partner is still a double touch (2v2)",
			rally:       RallyState{LastTeam: 0, LastPlayer: 0, BouncesOnSide: [2]int{0, 0}},
			event:       Event{Kind: EvBodyHit, Side: 0, Player: 1},
			wantWinner:  1,
			wantDecided: true,
		},
		{
			name:  "a body hit return by the receiving team (either partner) is a legal touch",
			rally: RallyState{LastTeam: 0, LastPlayer: 0, BouncesOnSide: [2]int{0, 0}},
			event: Event{Kind: EvBodyHit, Side: 1, Player: 3},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := c.rally
			winner, decided := judgeEvent(&r, c.event)
			if decided != c.wantDecided {
				t.Fatalf("decided: got %v, want %v", decided, c.wantDecided)
			}
			if decided && winner != c.wantWinner {
				t.Errorf("winner: got %d, want %d", winner, c.wantWinner)
			}
		})
	}
}

// TestJudgeFirstKick_BodyHitOpensServeBoxPending checks that an opening
// EvBodyHit (not just EvKick) also opens the serve-return box judge.
func TestJudgeFirstKick_BodyHitOpensServeBoxPending(t *testing.T) {
	r := RallyState{LastTeam: -1, LastPlayer: -1}
	judgeEvent(&r, Event{Kind: EvBodyHit, Side: 0, Player: 0})

	if !r.ServeBoxPending {
		t.Errorf("ServeBoxPending: got false, want true")
	}
	if r.LastTeam != 0 {
		t.Errorf("LastTeam: got %d, want 0", r.LastTeam)
	}
}

// --- scoring: win by 2, capped --------------------------------------------

func TestHasWonGame(t *testing.T) {
	const pointsToWin = 11
	cases := []struct {
		name               string
		winnerScore, loser int
		want               bool
	}{
		{"below PointsToWin never wins", 10, 0, false},
		{"first to PointsToWin with a 2+ lead wins", 11, 9, true},
		{"first to PointsToWin with exactly a 1 lead does not win yet (deuce)", 11, 10, false},
		{"deuce resolved by a 2-point lead past PointsToWin", 13, 11, true},
		{"still not a 2-point lead past PointsToWin", 12, 11, false},
		{"hits the hard deuce cap regardless of the 1-point lead", pointsToWin + deuceCap, pointsToWin + deuceCap - 1, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := hasWonGame(c.winnerScore, c.loser, pointsToWin); got != c.want {
				t.Errorf("hasWonGame(%d, %d, %d): got %v, want %v", c.winnerScore, c.loser, pointsToWin, got, c.want)
			}
		})
	}
}

func TestAwardPoint_DeuceRequiresTwoPointLead(t *testing.T) {
	s := newRulesTestState()
	s.Score = [2]int{10, 10}

	ev := awardPoint(&s, 0, nil)

	if s.Phase == PhaseGameOver {
		t.Fatalf("Phase: got PhaseGameOver, want PhasePoint (only a 1-point lead at 11-10)")
	}
	for _, e := range ev {
		if e.Kind == EvGameOver {
			t.Fatalf("got an EvGameOver at an 11-10 lead, want none")
		}
	}

	ev = applyRules(&s, nil) // drain PhasePoint's pause so the next point can be awarded
	for s.Phase == PhasePoint {
		ev = applyRules(&s, nil)
	}
	ev = awardPoint(&s, 0, ev)

	foundGameOver := false
	for _, e := range ev {
		if e.Kind == EvGameOver {
			foundGameOver = true
		}
	}
	if !foundGameOver || s.Phase != PhaseGameOver {
		t.Fatalf("got Score=%v Phase=%v, want GameOver at a 12-10 (2-point) lead", s.Score, s.Phase)
	}
}

func TestAwardPoint_CapAtPointsToWinPlusDeuceCap(t *testing.T) {
	s := newRulesTestState()
	s.Score = [2]int{s.Cfg.PointsToWin + deuceCap - 1, s.Cfg.PointsToWin + deuceCap - 2}

	ev := awardPoint(&s, 0, nil)

	if s.Phase != PhaseGameOver {
		t.Fatalf("Phase: got %v, want PhaseGameOver (hit the deuce cap)", s.Phase)
	}
	foundGameOver := false
	for _, e := range ev {
		if e.Kind == EvGameOver {
			foundGameOver = true
		}
	}
	if !foundGameOver {
		t.Fatalf("got %+v, want an EvGameOver", ev)
	}
}
