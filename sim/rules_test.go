package sim

import "testing"

func newRulesTestState() State {
	s := State{Cfg: DefaultConfig(), NumPlayers: 2}
	s.Players[0] = Player{Team: 0}
	s.Players[1] = Player{Team: 1}
	s.Phase = PhaseRally
	s.Rally.LastTeam = -1
	s.Rally.LastPlayer = -1
	s.LastServerPlayer = [2]int8{-1, -1}
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
			name:        "first kick of the rally sets LastTeam, decides nothing",
			rally:       RallyState{LastTeam: -1},
			event:       Event{Kind: EvKick, Side: 0, Player: 0},
			wantDecided: false,
		},
		{
			name:        "legal return by the other team decides nothing",
			rally:       RallyState{LastTeam: 0},
			event:       Event{Kind: EvKick, Side: 1, Player: 1},
			wantDecided: false,
		},
		{
			name:        "double touch before any receiver bounce: receiver wins",
			rally:       RallyState{LastTeam: 0, BouncesOnSide: [2]int{0, 0}},
			event:       Event{Kind: EvKick, Side: 0, Player: 0},
			wantWinner:  1,
			wantDecided: true,
		},
		{
			name:        "ball returns to kicker's team after a legal receiver bounce: kicker wins",
			rally:       RallyState{LastTeam: 0, BouncesOnSide: [2]int{0, 1}},
			event:       Event{Kind: EvKick, Side: 0, Player: 0},
			wantWinner:  0,
			wantDecided: true,
		},
		{
			name:        "a kick by the kicker's OWN partner is still a double touch (one touch per team)",
			rally:       RallyState{LastTeam: 0, LastPlayer: 0, BouncesOnSide: [2]int{0, 0}},
			event:       Event{Kind: EvKick, Side: 0, Player: 1},
			wantWinner:  1,
			wantDecided: true,
		},
		{
			name:        "first bounce on receiver side decides nothing",
			rally:       RallyState{LastTeam: 0, BouncesOnSide: [2]int{0, 0}},
			event:       Event{Kind: EvBounceFloor, Side: 1},
			wantDecided: false,
		},
		{
			name:        "second bounce on receiver side: kicker wins",
			rally:       RallyState{LastTeam: 0, BouncesOnSide: [2]int{0, 1}},
			event:       Event{Kind: EvBounceFloor, Side: 1},
			wantWinner:  0,
			wantDecided: true,
		},
		{
			name:        "bounce on kicker's own side before any receiver bounce: receiver wins",
			rally:       RallyState{LastTeam: 0, BouncesOnSide: [2]int{0, 0}},
			event:       Event{Kind: EvBounceFloor, Side: 0},
			wantWinner:  1,
			wantDecided: true,
		},
		{
			name:        "bounce on kicker's own side after a legal receiver bounce: kicker wins",
			rally:       RallyState{LastTeam: 0, BouncesOnSide: [2]int{0, 1}},
			event:       Event{Kind: EvBounceFloor, Side: 0},
			wantWinner:  0,
			wantDecided: true,
		},
		{
			name:        "wall before any bounce on the receiving side: receiver wins",
			rally:       RallyState{LastTeam: 0, BouncesOnSide: [2]int{0, 0}},
			event:       Event{Kind: EvHitWall, Side: 1},
			wantWinner:  1,
			wantDecided: true,
		},
		{
			name:        "wall after a legal bounce on the receiving side: legal, decides nothing",
			rally:       RallyState{LastTeam: 0, BouncesOnSide: [2]int{0, 1}},
			event:       Event{Kind: EvHitWall, Side: 1},
			wantDecided: false,
		},
		{
			name:        "wall on the kicker's own side is always legal",
			rally:       RallyState{LastTeam: 0, BouncesOnSide: [2]int{0, 0}},
			event:       Event{Kind: EvHitWall, Side: 0},
			wantDecided: false,
		},
		{
			name:        "ball goes out before any receiver bounce: receiver wins",
			rally:       RallyState{LastTeam: 0, BouncesOnSide: [2]int{0, 0}},
			event:       Event{Kind: EvOut, Side: 1},
			wantWinner:  1,
			wantDecided: true,
		},
		{
			name:        "ball goes out after a legal receiver bounce: kicker wins",
			rally:       RallyState{LastTeam: 0, BouncesOnSide: [2]int{0, 1}},
			event:       Event{Kind: EvOut, Side: 1},
			wantWinner:  0,
			wantDecided: true,
		},
		{
			name:        "net hit never decides the point",
			rally:       RallyState{LastTeam: 0, BouncesOnSide: [2]int{0, 0}},
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
	s.Rally.LastTeam = 0
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
	s.Rally.LastTeam = 0

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
	s.Rally.LastTeam = 0
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
	if s.Rally.LastTeam != -1 {
		t.Errorf("Rally.LastTeam: got %d, want -1 (fresh rally state)", s.Rally.LastTeam)
	}
}

// TestApplyRulesPlaygroundNeverScores covers ModePlayground's documented
// contract: no scoring, ever, regardless of what the ball does.
func TestApplyRulesPlaygroundNeverScores(t *testing.T) {
	s := newRulesTestState()
	s.Cfg.Mode = ModePlayground
	s.Rally.LastTeam = 0
	s.Rally.BouncesOnSide = [2]int{0, 1}

	ev := applyRules(&s, []Event{{Kind: EvBounceFloor, Side: 1}})

	if s.Score != [2]int{0, 0} {
		t.Fatalf("Score: got %v, want unchanged in Playground", s.Score)
	}
	for _, e := range ev {
		if e.Kind == EvPoint {
			t.Fatalf("got an EvPoint in Playground mode, want none: %+v", ev)
		}
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
		if s.Rally.ServerPlayer != 0 {
			t.Fatalf("Rally.ServerPlayer: got %d, want 0 (1v1: team == player index)", s.Rally.ServerPlayer)
		}
		assertFloat(t, "ball.Pos.X", s.Ball.Pos.X, -serveBallOffset)
		assertFloat(t, "player0.Pos.X", s.Players[0].Pos.X, -servePlayerOffset)
		assertFloat(t, "player1.Pos.X", s.Players[1].Pos.X, servePlayerOffset)

		// wave1-C: the resting pre-toss ball position is cosmetic only (the
		// real serve's legality comes from tossServe + the toss-bounce
		// judge, not from the legacy v1 KickReach field); what matters here
		// is that the server is positioned legally behind their own
		// service line, so a toss can ever land there.
		if !inServiceLineBounceZone(s.Players[0].Pos, 0, &s.Cfg) {
			t.Errorf("server0 at %+v is not behind its own service line (ServiceLineDist=%v)", s.Players[0].Pos, s.Cfg.ServiceLineDist)
		}
	})

	t.Run("side 1 serve", func(t *testing.T) {
		s := newRulesTestState()
		resetForServe(&s, 1)

		if s.Server != 1 {
			t.Fatalf("Server: got %d, want 1", s.Server)
		}
		assertFloat(t, "ball.Pos.X", s.Ball.Pos.X, serveBallOffset)

		if !inServiceLineBounceZone(s.Players[1].Pos, 1, &s.Cfg) {
			t.Errorf("server1 at %+v is not behind its own service line (ServiceLineDist=%v)", s.Players[1].Pos, s.Cfg.ServiceLineDist)
		}
	})

	// The partner now changes per SERVICE TURN, not per point (real padel,
	// §19.5): the same team serving again with no change of hands keeps
	// the same server; regaining the serve hands it to the partner.
	t.Run("2v2 places all 4 players and alternates the serving partner next turn", func(t *testing.T) {
		s := newRulesTestState()
		s.NumPlayers = 4
		s.LastServerPlayer = [2]int8{-1, -1}

		resetForServe(&s, 0)
		if s.Rally.ServerPlayer != 0 {
			t.Fatalf("first serve: ServerPlayer got %d, want 0", s.Rally.ServerPlayer)
		}
		for i, p := range s.Players {
			if p.Pos.X == 0 {
				t.Errorf("player %d was not positioned: %+v", i, p)
			}
		}

		resetForServe(&s, 0)
		if s.Rally.ServerPlayer != 0 {
			t.Fatalf("same turn: ServerPlayer got %d, want 0", s.Rally.ServerPlayer)
		}
		resetForServe(&s, 1)
		resetForServe(&s, 0)
		if s.Rally.ServerPlayer != 1 {
			t.Fatalf("team 0's next turn: ServerPlayer got %d, want 1 (alternate partner)", s.Rally.ServerPlayer)
		}
	})
}

func TestServeBoxForScore(t *testing.T) {
	cases := []struct {
		score [2]int
		want  int8
	}{
		{[2]int{0, 0}, 0},
		{[2]int{1, 0}, 1},
		{[2]int{1, 1}, 0},
		{[2]int{2, 1}, 1},
	}
	for _, c := range cases {
		if got := serveBoxForScore(c.score); got != c.want {
			t.Errorf("serveBoxForScore(%v): got %d, want %d", c.score, got, c.want)
		}
	}
}
