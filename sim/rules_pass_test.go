package sim

import "testing"

// 2v2 passes (futvóley): up to TouchLimit touches per team per crossing, never
// the same player twice in a row; the 1-bounce limit on a side holds for the
// whole possession (a pass does not grant a new bounce).

func doublesRally() RallyState {
	// Team 1 (player 2) just shot; team 0 is receiving.
	return RallyState{LastTeam: 1, LastPlayer: 2, TouchLimit: maxDoublesTouches}
}

func kick(side, player int8, tick uint32) Event {
	return Event{Kind: EvKick, Tick: tick, Side: side, Player: player}
}

func TestPass_ThreeTouchesAlternatingIsLegal(t *testing.T) {
	r := doublesRally()
	for i, p := range []int8{0, 1, 0} {
		if _, decided := judgeEvent(&r, kick(0, p, uint32(100+i*60))); decided {
			t.Fatalf("touch %d by player %d must be legal", i+1, p)
		}
	}
	if r.TeamTouches != 3 || r.LastTeam != 0 || r.LastPlayer != 0 {
		t.Fatalf("TeamTouches=%d LastTeam=%d LastPlayer=%d, want 3/0/0", r.TeamTouches, r.LastTeam, r.LastPlayer)
	}
}

func TestPass_FourthTouchIsFault(t *testing.T) {
	r := doublesRally()
	for i, p := range []int8{0, 1, 0} {
		judgeEvent(&r, kick(0, p, uint32(100+i*60)))
	}
	winner, decided := judgeEvent(&r, kick(0, 1, 400))
	if !decided || winner != 1 {
		t.Fatalf("4th touch: decided=%v winner=%d, want point for team 1", decided, winner)
	}
}

func TestPass_SamePlayerTwiceInARowIsFault(t *testing.T) {
	r := doublesRally()
	judgeEvent(&r, kick(0, 0, 100))
	winner, decided := judgeEvent(&r, kick(0, 0, 160))
	if !decided || winner != 1 {
		t.Fatalf("double touch: decided=%v winner=%d, want point for team 1", decided, winner)
	}
}

func TestPass_RivalTouchResetsTheCount(t *testing.T) {
	r := doublesRally()
	judgeEvent(&r, kick(0, 0, 100))
	judgeEvent(&r, kick(0, 1, 160))
	judgeEvent(&r, kick(1, 3, 260)) // crossed and returned
	if r.TeamTouches != 1 || r.LastTeam != 1 {
		t.Fatalf("after the rival touch TeamTouches=%d LastTeam=%d, want 1/1", r.TeamTouches, r.LastTeam)
	}
	if _, decided := judgeEvent(&r, kick(1, 2, 320)); decided {
		t.Fatal("the rival's own pass must be legal")
	}
}

func TestPass_OneBounceForTheWholePossession(t *testing.T) {
	cases := []struct {
		name        string
		events      []Event
		wantDecided bool
		wantWinner  int8
	}{
		{"bounce, pass, volley finish: legal", []Event{
			{Kind: EvBounceFloor, Side: 0}, kick(0, 0, 100), kick(0, 1, 160),
		}, false, 0},
		{"volley pass, bounce, finish: legal", []Event{
			kick(0, 0, 100), {Kind: EvBounceFloor, Side: 0}, kick(0, 1, 160),
		}, false, 0},
		{"bounce, pass, second bounce: point for the rival", []Event{
			{Kind: EvBounceFloor, Side: 0}, kick(0, 0, 100), {Kind: EvBounceFloor, Side: 0},
		}, true, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := doublesRally()
			var winner int8
			decided := false
			for _, e := range c.events {
				if winner, decided = judgeEvent(&r, e); decided {
					break
				}
			}
			if decided != c.wantDecided || (decided && winner != c.wantWinner) {
				t.Fatalf("decided=%v winner=%d, want %v/%d", decided, winner, c.wantDecided, c.wantWinner)
			}
		})
	}
}

func TestPass_ThirdTouchBouncingOnOwnSideIsFault(t *testing.T) {
	r := doublesRally()
	for i, p := range []int8{0, 1, 0} {
		judgeEvent(&r, kick(0, p, uint32(100+i*60)))
	}
	winner, decided := judgeEvent(&r, Event{Kind: EvBounceFloor, Side: 0})
	if !decided || winner != 1 {
		t.Fatalf("no touches left, own bounce: decided=%v winner=%d, want point for team 1", decided, winner)
	}
}

func TestPass_SinglesKeepsOneTouch(t *testing.T) {
	r := RallyState{LastTeam: 1, LastPlayer: 1, TouchLimit: 1}
	judgeEvent(&r, kick(0, 0, 100))
	if winner, decided := judgeEvent(&r, Event{Kind: EvBounceFloor, Side: 0}); !decided || winner != 1 {
		t.Fatalf("1v1 own-side bounce: decided=%v winner=%d, want point for team 1", decided, winner)
	}
}

func TestPass_ServingTeamCannotPassTheServe(t *testing.T) {
	r := RallyState{LastTeam: 0, LastPlayer: 0, TeamTouches: 1, TouchLimit: maxDoublesTouches, ServeBoxPending: true}
	if winner, decided := judgeEvent(&r, kick(0, 1, 100)); !decided || winner != 1 {
		t.Fatalf("server's partner touching the serve: decided=%v winner=%d, want point for team 1", decided, winner)
	}
}

func TestTouchLimitByMode(t *testing.T) {
	for _, c := range []struct {
		players int8
		want    uint8
	}{{2, 1}, {4, maxDoublesTouches}} {
		s := State{Cfg: DefaultConfig(), NumPlayers: c.players}
		resetForServe(&s, 0)
		if s.Rally.TouchLimit != c.want {
			t.Errorf("NumPlayers=%d: TouchLimit=%d, want %d", c.players, s.Rally.TouchLimit, c.want)
		}
		replayServe(&s, 0, 2)
		if s.Rally.TouchLimit != c.want {
			t.Errorf("NumPlayers=%d after replayServe: TouchLimit=%d, want %d", c.players, s.Rally.TouchLimit, c.want)
		}
	}
}

// Once the team's shot reached the rival side, the team cannot touch it
// again: it is no pass, and the point is the team's (the rival let it
// bounce and come back / the partner touched a winner at the net).
func TestPass_NoPassAfterTheBallCrossed(t *testing.T) {
	r := RallyState{LastTeam: 0, LastPlayer: 0, TeamTouches: 1, TouchLimit: maxDoublesTouches}
	r.BouncesOnSide[1] = 1
	if winner, decided := judgeEvent(&r, kick(0, 1, 200)); !decided || winner != 0 {
		t.Fatalf("partner touching a shot that already bounced on the rival side: decided=%v winner=%d, want point for team 0", decided, winner)
	}
	s := passState()
	s.Rally = r
	if CanTouch(&s, 1) || PassReady(&s, 1) {
		t.Fatal("CanTouch/PassReady must be false once the shot reached the rival side")
	}
}
