package sim

import "testing"

// --- post-strike serve-return judge (judgeServeReturn / ServeBoxPending) ---

// servedState returns a State already past a legal opening strike: Phase
// is PhaseRally, RallyState.ServeBoxPending is true (as judgeFirstKick
// would have set it), LastTeam is the server's team.
func servedState(server int8) State {
	s := newServeTestState(server)
	s.Phase = PhaseRally
	s.Rally.LastTeam = server
	s.Rally.LastPlayer = 0
	s.Rally.ServeTossBounced = true
	s.Rally.ServeBoxPending = true
	return s
}

func TestJudgeServeReturn_LegalBoxBounceClosesPendingAndCountsAsReceiverBounce(t *testing.T) {
	s := servedState(0)
	bounce := Event{Kind: EvBounceFloor, Side: 1, Pos: Vec3{X: 3, Z: 2}}

	handled, ev := judgeServeReturn(&s, bounce, nil)

	if !handled {
		t.Fatalf("handled: got false, want true")
	}
	if s.Rally.ServeBoxPending {
		t.Errorf("ServeBoxPending: got true, want false (closed by the legal box bounce)")
	}
	if s.Rally.BouncesOnSide[1] != 1 {
		t.Errorf("BouncesOnSide[1]: got %d, want 1", s.Rally.BouncesOnSide[1])
	}
	for _, e := range ev {
		if e.Kind == EvServeFault || e.Kind == EvLet {
			t.Errorf("got %+v, want no fault/let for a legal box bounce", ev)
		}
	}
}

func TestJudgeServeReturn_BounceOutsideBoxIsFault(t *testing.T) {
	s := servedState(0)
	// Lands on the receiving side, but past the service line: outside the
	// diagonal box.
	bounce := Event{Kind: EvBounceFloor, Side: 1, Pos: Vec3{X: s.Cfg.ServiceLineDist + 1, Z: 2}}

	handled, ev := judgeServeReturn(&s, bounce, nil)

	if !handled {
		t.Fatalf("handled: got false, want true")
	}
	assertServeFaultThenRetry(t, &s, ev)
}

func TestJudgeServeReturn_BounceOnServersOwnSideIsFault(t *testing.T) {
	s := servedState(0)
	bounce := Event{Kind: EvBounceFloor, Side: 0, Pos: Vec3{X: -3}}

	handled, ev := judgeServeReturn(&s, bounce, nil)

	if !handled {
		t.Fatalf("handled: got false, want true")
	}
	assertServeFaultThenRetry(t, &s, ev)
}

func TestJudgeServeReturn_WallOnReceiverSideBeforeBounceIsFault(t *testing.T) {
	s := servedState(0)
	handled, ev := judgeServeReturn(&s, Event{Kind: EvHitWall, Side: 1}, nil)

	if !handled {
		t.Fatalf("handled: got false, want true")
	}
	assertServeFaultThenRetry(t, &s, ev)
}

func TestJudgeServeReturn_WallOnServersOwnSideIsHarmless(t *testing.T) {
	s := servedState(0)
	handled, ev := judgeServeReturn(&s, Event{Kind: EvHitWall, Side: 0}, nil)

	if !handled {
		t.Fatalf("handled: got false, want true")
	}
	if len(ev) != 0 {
		t.Fatalf("got %+v, want no events for a harmless own-side wall touch", ev)
	}
	if !s.Rally.ServeBoxPending {
		t.Errorf("ServeBoxPending: got false, want still true (nothing decided yet)")
	}
}

func TestJudgeServeReturn_OutIsFault(t *testing.T) {
	s := servedState(0)
	handled, ev := judgeServeReturn(&s, Event{Kind: EvOut, Side: 1}, nil)

	if !handled {
		t.Fatalf("handled: got false, want true")
	}
	assertServeFaultThenRetry(t, &s, ev)
}

func TestJudgeServeReturn_NetTouchOnReceiverSideDoesNotSetFlag(t *testing.T) {
	s := servedState(0)
	// The net touch happened on the RECEIVING side (the ball already
	// crossed), so it is not the serve's own pre-bounce net clip: only an
	// own-side (server's side) net touch can turn an eventual legal bounce
	// into a Let.
	handled, ev := judgeServeReturn(&s, Event{Kind: EvHitNet, Side: 1}, nil)

	if !handled {
		t.Fatalf("handled: got false, want true")
	}
	if len(ev) != 0 {
		t.Fatalf("got %+v, want no events", ev)
	}
	if s.Rally.ServeNetTouch {
		t.Errorf("ServeNetTouch: got true, want false (net touch was not on the server's own side)")
	}
}

// TestJudgeServeReturn_UnrelatedKindFallsThrough checks that an event kind
// judgeServeReturn does not special-case (e.g. a second EvKick/EvBodyHit
// during the serve's own return flight) is left for the generic
// judgeRally/judgeEvent one-touch judge instead of being silently
// swallowed.
func TestJudgeServeReturn_UnrelatedKindFallsThrough(t *testing.T) {
	s := servedState(0)
	handled, ev := judgeServeReturn(&s, Event{Kind: EvKick, Side: 0, Player: 0}, nil)

	if handled {
		t.Fatalf("handled: got true, want false (EvKick is not judgeServeReturn's concern)")
	}
	if len(ev) != 0 {
		t.Fatalf("got %+v, want ev returned unchanged", ev)
	}
}

func TestJudgeServeReturn_NetTouchAlone_DecidesNothingYet(t *testing.T) {
	s := servedState(0)
	handled, ev := judgeServeReturn(&s, Event{Kind: EvHitNet, Side: 0}, nil)

	if !handled {
		t.Fatalf("handled: got false, want true")
	}
	if len(ev) != 0 {
		t.Fatalf("got %+v, want no events yet (net touch alone never decides)", ev)
	}
	if !s.Rally.ServeNetTouch {
		t.Errorf("ServeNetTouch: got false, want true")
	}
}

// TestJudgeServeReturn_LetOnNetTouchThenLegalBoxBounce is the doc's Let
// case (docs/design/contact-v2.md §7.5): the serve clips the net but still
// lands in the correct box, so the attempt replays without counting as a
// fault.
func TestJudgeServeReturn_LetOnNetTouchThenLegalBoxBounce(t *testing.T) {
	s := servedState(0)
	_, ev := judgeServeReturn(&s, Event{Kind: EvHitNet, Side: 0}, nil)

	bounce := Event{Kind: EvBounceFloor, Side: 1, Pos: Vec3{X: 3, Z: 2}}
	handled, ev2 := judgeServeReturn(&s, bounce, ev)

	if !handled {
		t.Fatalf("handled: got false, want true")
	}
	foundLet := false
	for _, e := range ev2 {
		if e.Kind == EvLet {
			foundLet = true
		}
		if e.Kind == EvServeFault || e.Kind == EvPoint {
			t.Fatalf("got %+v, want a Let, not a fault/point", ev2)
		}
	}
	if !foundLet {
		t.Fatalf("got %+v, want an EvLet", ev2)
	}
	if s.Phase != PhaseServe {
		t.Errorf("Phase: got %v, want PhaseServe (the let replays the point)", s.Phase)
	}
	if s.Rally.ServeAttempt != 1 {
		t.Errorf("Rally.ServeAttempt: got %d, want unchanged at 1 (a let is not a fault)", s.Rally.ServeAttempt)
	}
}

// TestLetServe_PreservesTheSecondServeAttemptNumber checks that a let
// during the SECOND serve attempt replays as attempt 2, not resetting back
// to a fresh first serve.
func TestLetServe_PreservesTheSecondServeAttemptNumber(t *testing.T) {
	s := servedState(0)
	s.Rally.ServeAttempt = 2
	s.Rally.ServeNetTouch = true

	bounce := Event{Kind: EvBounceFloor, Side: 1, Pos: Vec3{X: 3, Z: 2}}
	_, ev := judgeServeReturn(&s, bounce, nil)

	foundLet := false
	for _, e := range ev {
		if e.Kind == EvLet {
			foundLet = true
		}
	}
	if !foundLet {
		t.Fatalf("got %+v, want an EvLet", ev)
	}
	if s.Rally.ServeAttempt != 2 {
		t.Errorf("Rally.ServeAttempt: got %d, want unchanged at 2", s.Rally.ServeAttempt)
	}
}

// --- replayServe keeps the server/box fixed within one point ---------------

func TestReplayServe_KeepsServerPlayerAndBoxFixed(t *testing.T) {
	s := newServeTestState(1)
	s.Rally.ServeBox = 1
	s.Rally.ServerPlayer = 1

	replayServe(&s, 1, 2)

	if s.Rally.ServeBox != 1 {
		t.Errorf("ServeBox: got %d, want unchanged at 1", s.Rally.ServeBox)
	}
	if s.Rally.ServerPlayer != 1 {
		t.Errorf("ServerPlayer: got %d, want unchanged at 1", s.Rally.ServerPlayer)
	}
	if s.Rally.ServeAttempt != 2 {
		t.Errorf("ServeAttempt: got %d, want 2", s.Rally.ServeAttempt)
	}
	if s.Phase != PhaseServe {
		t.Errorf("Phase: got %v, want PhaseServe", s.Phase)
	}
	if s.Rally.LastTeam != -1 || s.Rally.LastPlayer != -1 {
		t.Errorf("LastTeam/LastPlayer: got %d/%d, want -1/-1 (fresh attempt)", s.Rally.LastTeam, s.Rally.LastPlayer)
	}
	if s.Rally.ServeTossBounced || s.Rally.ServeBoxPending || s.Rally.ServeNetTouch {
		t.Errorf("serve judge flags: got %+v, want all false (fresh attempt)", s.Rally)
	}
}
