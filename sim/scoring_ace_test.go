package sim

import "testing"

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
		{Kind: EvKick, Side: 0, Player: 0},                     // server's opening strike
		{Kind: EvBounceFloor, Side: 1, Pos: Vec3{X: 3, Z: -1}}, // legal landing in box 0 (receiver team 1's right = -Z, diagonal fix)
	})
	// First bounce only legalizes the box; nothing decided yet.
	for _, e := range ev {
		if e.Kind == EvAce || e.Kind == EvPoint {
			t.Fatalf("got %v too early (only the serve's own legal bounce happened): %+v", e.Kind, ev)
		}
	}

	ev = judgeRally(&s, []Event{
		{Kind: EvBounceFloor, Side: 1, Pos: Vec3{X: 3, Z: -1}}, // second bounce, still untouched
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
		{Kind: EvBounceFloor, Side: 1, Pos: Vec3{X: 3, Z: -1}},
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
