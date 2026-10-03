package sim

import "testing"

// A body graze followed by the same player's strike inside bodyMergeTicks is
// ONE touch (the capsule meets the ball a few ticks before the foot); a
// later strike, or a teammate's, is still a double touch.
func TestJudgeEvent_BodyGrazeThenOwnStrike(t *testing.T) {
	cases := []struct {
		name        string
		kickPlayer  int8
		kickDelay   uint32
		wantDecided bool
	}{
		{"same player right after the graze: one touch", 0, 3, false},
		{"same player at the window edge: one touch", 0, bodyMergeTicks, false},
		{"same player after the window: double touch", 0, bodyMergeTicks + 1, true},
		{"teammate right after the graze: double touch", 1, 2, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := RallyState{LastTeam: 1, LastPlayer: 2}
			if _, decided := judgeEvent(&r, Event{Kind: EvBodyHit, Tick: 100, Side: 0, Player: 0}); decided {
				t.Fatal("the graze itself must not decide the point")
			}
			winner, decided := judgeEvent(&r, Event{Kind: EvKick, Tick: 100 + c.kickDelay, Side: 0, Player: c.kickPlayer})
			if decided != c.wantDecided {
				t.Fatalf("decided=%v, want %v", decided, c.wantDecided)
			}
			if decided && winner != 1 {
				t.Errorf("winner=%d, want 1 (double touch by team 0)", winner)
			}
		})
	}
}

// A strike followed by a graze of the same player's body (after the grace
// period) is still a double touch: the merge only forgives graze-then-strike.
func TestJudgeEvent_StrikeThenBodyGrazeIsDoubleTouch(t *testing.T) {
	r := RallyState{LastTeam: 1, LastPlayer: 2}
	judgeEvent(&r, Event{Kind: EvKick, Tick: 100, Side: 0, Player: 0})
	if _, decided := judgeEvent(&r, Event{Kind: EvBodyHit, Tick: 140, Side: 0, Player: 0}); !decided {
		t.Fatal("strike then body hit by the same team must be a double touch")
	}
}
