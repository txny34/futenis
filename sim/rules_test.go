package sim

import "testing"

func newRulesTestState() State {
	s := State{Cfg: DefaultConfig()}
	s.Players[0] = Player{Side: 0}
	s.Players[1] = Player{Side: 1}
	s.Phase = PhaseRally
	s.Rally.LastKicker = -1
	return s
}

// TestJudgeEvent is the table-driven core of the rules contract: for a given
// RallyState and a single incoming event, who wins the point (if anyone)?
func TestJudgeEvent(t *testing.T) {
	cases := []struct {
		name        string
		rally       RallyState
		event       Event
		wantWinner  int8
		wantDecided bool
	}{
		{
			name:        "first kick of the rally sets LastKicker, decides nothing",
			rally:       RallyState{LastKicker: -1},
			event:       Event{Kind: EvKick, Side: 0},
			wantDecided: false,
		},
		{
			name:        "legal return by the other side decides nothing",
			rally:       RallyState{LastKicker: 0},
			event:       Event{Kind: EvKick, Side: 1},
			wantDecided: false,
		},
		{
			name:        "double touch before any receiver bounce: receiver wins",
			rally:       RallyState{LastKicker: 0, BouncesOnSide: [2]int{0, 0}},
			event:       Event{Kind: EvKick, Side: 0},
			wantWinner:  1,
			wantDecided: true,
		},
		{
			name:        "ball returns to kicker's side after a legal receiver bounce: kicker wins",
			rally:       RallyState{LastKicker: 0, BouncesOnSide: [2]int{0, 1}},
			event:       Event{Kind: EvKick, Side: 0},
			wantWinner:  0,
			wantDecided: true,
		},
		{
			name:        "first bounce on receiver side decides nothing",
			rally:       RallyState{LastKicker: 0, BouncesOnSide: [2]int{0, 0}},
			event:       Event{Kind: EvBounceFloor, Side: 1},
			wantDecided: false,
		},
		{
			name:        "second bounce on receiver side: kicker wins",
			rally:       RallyState{LastKicker: 0, BouncesOnSide: [2]int{0, 1}},
			event:       Event{Kind: EvBounceFloor, Side: 1},
			wantWinner:  0,
			wantDecided: true,
		},
		{
			name:        "bounce on kicker's own side before any receiver bounce: receiver wins",
			rally:       RallyState{LastKicker: 0, BouncesOnSide: [2]int{0, 0}},
			event:       Event{Kind: EvBounceFloor, Side: 0},
			wantWinner:  1,
			wantDecided: true,
		},
		{
			name:        "bounce on kicker's own side after a legal receiver bounce: kicker wins",
			rally:       RallyState{LastKicker: 0, BouncesOnSide: [2]int{0, 1}},
			event:       Event{Kind: EvBounceFloor, Side: 0},
			wantWinner:  0,
			wantDecided: true,
		},
		{
			name:        "wall before any bounce on the receiving side: receiver wins",
			rally:       RallyState{LastKicker: 0, BouncesOnSide: [2]int{0, 0}},
			event:       Event{Kind: EvHitWall, Side: 1},
			wantWinner:  1,
			wantDecided: true,
		},
		{
			name:        "wall after a legal bounce on the receiving side: legal, decides nothing",
			rally:       RallyState{LastKicker: 0, BouncesOnSide: [2]int{0, 1}},
			event:       Event{Kind: EvHitWall, Side: 1},
			wantDecided: false,
		},
		{
			name:        "wall on the kicker's own side is always legal",
			rally:       RallyState{LastKicker: 0, BouncesOnSide: [2]int{0, 0}},
			event:       Event{Kind: EvHitWall, Side: 0},
			wantDecided: false,
		},
		{
			name:        "ball goes out before any receiver bounce: receiver wins",
			rally:       RallyState{LastKicker: 0, BouncesOnSide: [2]int{0, 0}},
			event:       Event{Kind: EvOut, Side: 1},
			wantWinner:  1,
			wantDecided: true,
		},
		{
			name:        "ball goes out after a legal receiver bounce: kicker wins",
			rally:       RallyState{LastKicker: 0, BouncesOnSide: [2]int{0, 1}},
			event:       Event{Kind: EvOut, Side: 1},
			wantWinner:  0,
			wantDecided: true,
		},
		{
			name:        "net hit never decides the point",
			rally:       RallyState{LastKicker: 0, BouncesOnSide: [2]int{0, 0}},
			event:       Event{Kind: EvHitNet, Side: 0},
			wantDecided: false,
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

func TestApplyRulesAwardsPointAndPauses(t *testing.T) {
	s := newRulesTestState()
	s.Rally.LastKicker = 0
	s.Rally.BouncesOnSide = [2]int{0, 1}

	ev := applyRules(&s, []Event{{Kind: EvBounceFloor, Side: 1}})

	if s.Score[0] != 1 {
		t.Fatalf("Score[0]: got %d, want 1", s.Score[0])
	}
	if s.Phase != PhasePoint {
		t.Fatalf("Phase: got %v, want PhasePoint", s.Phase)
	}
	if s.Rally.PauseTicks != PointPauseTicks {
		t.Errorf("PauseTicks: got %d, want %d", s.Rally.PauseTicks, PointPauseTicks)
	}
	if s.Server != 0 {
		t.Errorf("Server: got %d, want 0 (the winner serves next)", s.Server)
	}

	foundPoint := false
	for _, e := range ev {
		if e.Kind == EvPoint && e.Side == 0 {
			foundPoint = true
		}
	}
	if !foundPoint {
		t.Fatalf("got %+v, want an EvPoint with Side 0", ev)
	}
}

func TestApplyRulesStopsAtFirstDecisiveEvent(t *testing.T) {
	s := newRulesTestState()
	s.Rally.LastKicker = 0

	// Two events in one tick: a net hit (never decisive) followed by an out
	// on the kicker's own side before any receiver bounce (receiver wins).
	// Rules must stop judging after the first decisive event and must not
	// re-judge the EvPoint it appends.
	ev := applyRules(&s, []Event{
		{Kind: EvHitNet, Side: 0},
		{Kind: EvOut, Side: 0},
	})

	if s.Score[1] != 1 {
		t.Fatalf("Score[1]: got %d, want 1", s.Score[1])
	}
	if len(ev) != 3 {
		t.Fatalf("got %d events, want 3 (net, out, point)", len(ev))
	}
}

func TestApplyRulesGameOver(t *testing.T) {
	s := newRulesTestState()
	s.Rally.LastKicker = 0
	s.Rally.BouncesOnSide = [2]int{0, 1}
	s.Score[0] = s.Cfg.PointsToWin - 1

	ev := applyRules(&s, []Event{{Kind: EvBounceFloor, Side: 1}})

	if s.Phase != PhaseGameOver {
		t.Fatalf("Phase: got %v, want PhaseGameOver", s.Phase)
	}
	if s.Score[0] != s.Cfg.PointsToWin {
		t.Fatalf("Score[0]: got %d, want %d", s.Score[0], s.Cfg.PointsToWin)
	}

	foundGameOver := false
	for _, e := range ev {
		if e.Kind == EvGameOver {
			foundGameOver = true
			if e.Side != 0 {
				t.Errorf("EvGameOver.Side: got %d, want 0", e.Side)
			}
		}
	}
	if !foundGameOver {
		t.Fatalf("got %+v, want an EvGameOver", ev)
	}
}

func TestApplyRulesPointPauseThenReserve(t *testing.T) {
	s := newRulesTestState()
	s.Phase = PhasePoint
	s.Server = 1
	s.Rally.PauseTicks = 2

	applyRules(&s, nil)
	if s.Phase != PhasePoint {
		t.Fatalf("Phase: got %v, want still PhasePoint (1 tick left)", s.Phase)
	}
	if s.Rally.PauseTicks != 1 {
		t.Fatalf("PauseTicks: got %d, want 1", s.Rally.PauseTicks)
	}

	applyRules(&s, nil)
	if s.Phase != PhaseServe {
		t.Fatalf("Phase: got %v, want PhaseServe after the pause elapses", s.Phase)
	}
	if s.Server != 1 {
		t.Errorf("Server: got %d, want 1 (winner serves)", s.Server)
	}
	if s.Rally.LastKicker != -1 {
		t.Errorf("Rally.LastKicker: got %d, want -1 (fresh rally state)", s.Rally.LastKicker)
	}
}

func TestResetForServe(t *testing.T) {
	t.Run("side 0 serve", func(t *testing.T) {
		s := newRulesTestState()
		resetForServe(&s, 0)

		if s.Phase != PhaseServe {
			t.Fatalf("Phase: got %v, want PhaseServe", s.Phase)
		}
		if s.Server != 0 {
			t.Fatalf("Server: got %d, want 0", s.Server)
		}
		assertFloat(t, "ball.Pos.X", s.Ball.Pos.X, -serveBallOffset)
		assertFloat(t, "player0.Pos.X", s.Players[0].Pos.X, -servePlayerOffset)
		assertFloat(t, "player1.Pos.X", s.Players[1].Pos.X, servePlayerOffset)

		dist := s.Ball.Pos.Sub(s.Players[0].Pos).Len()
		if dist > s.Cfg.KickReach {
			t.Errorf("ball is %v from the server, want <= KickReach (%v)", dist, s.Cfg.KickReach)
		}
	})

	t.Run("side 1 serve", func(t *testing.T) {
		s := newRulesTestState()
		resetForServe(&s, 1)

		if s.Server != 1 {
			t.Fatalf("Server: got %d, want 1", s.Server)
		}
		assertFloat(t, "ball.Pos.X", s.Ball.Pos.X, serveBallOffset)

		dist := s.Ball.Pos.Sub(s.Players[1].Pos).Len()
		if dist > s.Cfg.KickReach {
			t.Errorf("ball is %v from the server, want <= KickReach (%v)", dist, s.Cfg.KickReach)
		}
	})
}
