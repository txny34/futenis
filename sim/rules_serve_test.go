package sim

import "testing"

// --- service box / toss-bounce geometry ------------------------------------

func TestServeBoxXRange(t *testing.T) {
	cfg := DefaultConfig()
	cases := []struct {
		server           int8
		wantMin, wantMax float64
	}{
		{0, 0, cfg.ServiceLineDist},
		{1, -cfg.ServiceLineDist, 0},
	}
	for _, c := range cases {
		min, max := serveBoxXRange(c.server, &cfg)
		assertFloat(t, "min", min, c.wantMin)
		assertFloat(t, "max", max, c.wantMax)
	}
}

// TestServeBoxZRange: box 0 is the RECEIVER's right (diagonal serve), so
// the physical Z half depends on who serves. Previously box 0 was +Z for
// both servers, which made team 0's serve go down the line, not diagonal.
func TestServeBoxZRange(t *testing.T) {
	cases := []struct {
		server, box      int8
		wantMin, wantMax float64
	}{
		{0, 0, -CourtHalfWidth, 0},
		{0, 1, 0, CourtHalfWidth},
		{1, 0, 0, CourtHalfWidth},
		{1, 1, -CourtHalfWidth, 0},
	}
	for _, c := range cases {
		min, max := serveBoxZRange(c.server, c.box)
		assertFloat(t, "min", min, c.wantMin)
		assertFloat(t, "max", max, c.wantMax)
	}
}

// TestInServeBox covers every (server, box) combination on both sides of
// the net, both diagonal boxes, plus landing spots that must be rejected
// (wrong side, wrong box, past the service line, past the sideline).
func TestInServeBox(t *testing.T) {
	cfg := DefaultConfig()
	cases := []struct {
		name   string
		pos    Vec3
		server int8
		box    int8
		want   bool
	}{
		// Server 0's box 0 is team 1's right = -Z (diagonal; was +Z before the fix).
		{"server0 box0: center of the box is legal", Vec3{X: 3, Z: -2}, 0, 0, true},
		{"server0 box0: on the box boundaries is legal", Vec3{X: cfg.ServiceLineDist, Z: -CourtHalfWidth}, 0, 0, true},
		{"server0 box0: net-line edge (X=0) is legal", Vec3{X: 0, Z: -1}, 0, 0, true},
		{"server0 box0: wrong Z half (box 1 territory) is illegal", Vec3{X: 3, Z: 2}, 0, 0, false},
		{"server0 box0: past the service line is illegal", Vec3{X: cfg.ServiceLineDist + 0.01, Z: -2}, 0, 0, false},
		{"server0 box0: on the server's own side is illegal", Vec3{X: -3, Z: -2}, 0, 0, false},
		{"server0 box1: positive Z half is legal", Vec3{X: 3, Z: 2}, 0, 1, true},
		{"server1 box0: negative X half, Z>=0 is legal", Vec3{X: -3, Z: 2}, 1, 0, true},
		{"server1 box0: positive X (server's own side) is illegal", Vec3{X: 3, Z: 2}, 1, 0, false},
		{"server1 box1: negative X, negative Z is legal", Vec3{X: -3, Z: -2}, 1, 1, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := inServeBox(c.pos, c.server, c.box, &cfg); got != c.want {
				t.Errorf("inServeBox(%+v, server=%d, box=%d): got %v, want %v", c.pos, c.server, c.box, got, c.want)
			}
		})
	}
}

func TestInServiceLineBounceZone(t *testing.T) {
	cfg := DefaultConfig()
	cases := []struct {
		name   string
		pos    Vec3
		server int8
		want   bool
	}{
		{"behind the line on the server's own side is legal", Vec3{X: -7.5}, 0, true},
		{"exactly on the line is illegal (strict >)", Vec3{X: -cfg.ServiceLineDist}, 0, false},
		{"short of the line on the server's own side is illegal", Vec3{X: -3}, 0, false},
		{"on the receiver's side entirely is illegal", Vec3{X: 7.5}, 0, false},
		{"server1 mirrors server0", Vec3{X: 7.5}, 1, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := inServiceLineBounceZone(c.pos, c.server, &cfg); got != c.want {
				t.Errorf("inServiceLineBounceZone(%+v, server=%d): got %v, want %v", c.pos, c.server, got, c.want)
			}
		})
	}
}

// --- pre-strike toss-bounce judge (judgeServe) ------------------------------

func newServeTestState(server int8) State {
	s := newRulesTestState()
	s.Server = server
	resetForServe(&s, server)
	return s
}

func TestJudgeServe_LegalTossBounceSetsFlagNoEvent(t *testing.T) {
	s := newServeTestState(0)
	bounce := Event{Kind: EvBounceFloor, Side: 0, Pos: Vec3{X: -7.5}}

	ev := judgeServe(&s, []Event{bounce})

	if !s.Rally.ServeTossBounced {
		t.Fatalf("ServeTossBounced: got false, want true after a legal toss bounce")
	}
	if len(ev) != 1 {
		t.Fatalf("got %d events, want 1 (no fault appended): %+v", len(ev), ev)
	}
	if s.Phase != PhaseServe {
		t.Fatalf("Phase: got %v, want still PhaseServe", s.Phase)
	}
	if s.Rally.ServeAttempt != 1 {
		t.Errorf("ServeAttempt: got %d, want unchanged at 1", s.Rally.ServeAttempt)
	}
}

func TestJudgeServe_TossBounceShortOfLineIsFault(t *testing.T) {
	s := newServeTestState(0)
	bounce := Event{Kind: EvBounceFloor, Side: 0, Pos: Vec3{X: -3}} // short of ServiceLineDist

	ev := judgeServe(&s, []Event{bounce})

	assertServeFaultThenRetry(t, &s, ev)
}

func TestJudgeServe_SecondBounceBeforeStrikeIsFault(t *testing.T) {
	s := newServeTestState(0)
	s.Rally.ServeTossBounced = true
	bounce := Event{Kind: EvBounceFloor, Side: 0, Pos: Vec3{X: -7.5}}

	ev := judgeServe(&s, []Event{bounce})

	assertServeFaultThenRetry(t, &s, ev)
}

func TestJudgeServe_WallBeforeBounceIsFault(t *testing.T) {
	s := newServeTestState(0)
	ev := judgeServe(&s, []Event{{Kind: EvHitWall, Side: 0}})
	assertServeFaultThenRetry(t, &s, ev)
}

func TestJudgeServe_WhiffIsFault(t *testing.T) {
	s := newServeTestState(0)
	ev := judgeServe(&s, []Event{{Kind: EvWhiff, Side: 0}})
	assertServeFaultThenRetry(t, &s, ev)
}

func TestJudgeServe_OutIsFault(t *testing.T) {
	s := newServeTestState(0)
	ev := judgeServe(&s, []Event{{Kind: EvOut, Side: 0}})
	assertServeFaultThenRetry(t, &s, ev)
}

// TestJudgeServe_DoubleFaultAwardsPoint drives two consecutive fault
// attempts: the first replays the point (ServeAttempt 1 -> 2), the second
// is a double fault that awards the point to the receiver.
func TestJudgeServe_DoubleFaultAwardsPoint(t *testing.T) {
	s := newServeTestState(0)

	ev := judgeServe(&s, []Event{{Kind: EvWhiff, Side: 0}})
	assertServeFaultThenRetry(t, &s, ev)

	ev = judgeServe(&s, []Event{{Kind: EvWhiff, Side: 0}})

	foundFault, foundPoint := false, false
	for _, e := range ev {
		if e.Kind == EvServeFault {
			foundFault = true
		}
		if e.Kind == EvPoint {
			foundPoint = true
			if e.Side != 1 {
				t.Errorf("EvPoint.Side: got %d, want 1 (the receiver, after a double fault)", e.Side)
			}
		}
	}
	if !foundFault || !foundPoint {
		t.Fatalf("got %+v, want both EvServeFault and EvPoint(side=1)", ev)
	}
	if s.Score[1] != 1 {
		t.Errorf("Score[1]: got %d, want 1", s.Score[1])
	}
	if s.Phase != PhasePoint {
		t.Errorf("Phase: got %v, want PhasePoint", s.Phase)
	}
}

// assertServeFaultThenRetry is the shared assertion for every single-fault
// (not double-fault) case above: EvServeFault is emitted, the point is NOT
// decided, and the state is reset for a second serve attempt by the same
// server, same box.
func assertServeFaultThenRetry(t *testing.T, s *State, ev []Event) {
	t.Helper()
	foundFault := false
	for _, e := range ev {
		if e.Kind == EvServeFault {
			foundFault = true
			if e.Side != 0 {
				t.Errorf("EvServeFault.Side: got %d, want 0", e.Side)
			}
		}
		if e.Kind == EvPoint {
			t.Fatalf("got an EvPoint on the first fault, want none (only a double fault decides): %+v", ev)
		}
	}
	if !foundFault {
		t.Fatalf("got %+v, want an EvServeFault", ev)
	}
	if s.Phase != PhaseServe {
		t.Errorf("Phase: got %v, want PhaseServe (retrying)", s.Phase)
	}
	if s.Rally.ServeAttempt != 2 {
		t.Errorf("Rally.ServeAttempt: got %d, want 2 (second serve)", s.Rally.ServeAttempt)
	}
	if s.Rally.LastTeam != -1 {
		t.Errorf("Rally.LastTeam: got %d, want -1 (fresh attempt)", s.Rally.LastTeam)
	}
}

// --- struck before the toss bounce (judgeRally's pre-check) ----------------

func TestJudgeRally_StruckBeforeTossBounceIsFault(t *testing.T) {
	s := newServeTestState(0)
	// The strike pipeline (strike.go) already flipped Phase to PhaseRally
	// for this EvKick by the time applyRules/judgeRally sees it, even
	// though the mandatory toss bounce never happened.
	s.Phase = PhaseRally

	ev := judgeRally(&s, []Event{{Kind: EvKick, Side: 0, Player: 0}})

	assertServeFaultThenRetry(t, &s, ev)
}

func TestJudgeRally_BodyHitBeforeTossBounceIsFault(t *testing.T) {
	s := newServeTestState(0)
	s.Phase = PhaseRally

	ev := judgeRally(&s, []Event{{Kind: EvBodyHit, Side: 0, Player: 0}})

	assertServeFaultThenRetry(t, &s, ev)
}

// TestJudgeRally_VolleyReturnDuringServeBoxPendingForfeitsPointToServer
// exercises judgeRally's full dispatch chain end to end: real padel
// requires the serve to bounce once in the receiver's diagonal box before
// anyone may play it. A volley return (the receiver kicks before any
// bounce) is the RECEIVER's fault and must award the point to the server's
// team immediately -- not a serve fault/replay (the server did nothing
// wrong), and not a silent legal return either.
func TestJudgeRally_VolleyReturnDuringServeBoxPendingForfeitsPointToServer(t *testing.T) {
	s := newServeTestState(0)
	s.Rally.ServeTossBounced = true
	s.Phase = PhaseRally

	ev := judgeRally(&s, []Event{{Kind: EvKick, Side: 0, Player: 0}, {Kind: EvKick, Side: 1, Player: 1}})

	foundPoint, foundFault := false, false
	for _, e := range ev {
		switch e.Kind {
		case EvPoint:
			foundPoint = true
			if e.Side != 0 {
				t.Errorf("EvPoint.Side: got %d, want 0 (the server)", e.Side)
			}
		case EvServeFault:
			foundFault = true
		}
	}
	if !foundPoint {
		t.Fatalf("got %+v, want an EvPoint(side=0)", ev)
	}
	if foundFault {
		t.Fatalf("got an EvServeFault for the receiver's early volley, want none (not the server's fault): %+v", ev)
	}
	if s.Rally.ServerPlayer != 0 || s.Server != 0 {
		t.Errorf("ServerPlayer/Server: got %d/%d, want unchanged (no replay, this was a forfeited point)", s.Rally.ServerPlayer, s.Server)
	}
}

// TestJudgeServeVolley_OwnTeamTouchFallsThroughToGenericDoubleTouch covers
// the other half of judgeServeVolley: a touch by the SERVER's own team
// (e.g. an own body hit) while the return is still pending must NOT be
// swallowed as a forfeit -- it falls through to the generic one-touch
// judge, which treats it as a double touch (receiver wins).
func TestJudgeServeVolley_OwnTeamTouchFallsThroughToGenericDoubleTouch(t *testing.T) {
	s := newServeTestState(0)
	s.Rally.ServeTossBounced = true
	s.Phase = PhaseRally

	ev := judgeRally(&s, []Event{{Kind: EvKick, Side: 0, Player: 0}, {Kind: EvBodyHit, Side: 0, Player: 0}})

	foundPoint := false
	for _, e := range ev {
		if e.Kind == EvPoint {
			foundPoint = true
			if e.Side != 1 {
				t.Errorf("EvPoint.Side: got %d, want 1 (receiver wins a double touch by the server's own team)", e.Side)
			}
		}
	}
	if !foundPoint {
		t.Fatalf("got %+v, want an EvPoint(side=1)", ev)
	}
}

// TestJudgeRally_ServeBoxFaultStopsAtFirstDecisiveEvent exercises
// judgeRally's own ServeBoxPending branch end to end (not just
// judgeServeReturn in isolation): a serve-return fault must leave
// PhaseRally and stop the loop, just like the generic one-touch judge
// already does for a normal rally fault.
func TestJudgeRally_ServeBoxFaultStopsAtFirstDecisiveEvent(t *testing.T) {
	s := newServeTestState(0)
	s.Rally.ServeTossBounced = true
	s.Phase = PhaseRally

	// The opening strike (sets ServeBoxPending), then its own bounce lands
	// outside the diagonal box: a serve fault, so the trailing EvHitWall
	// below must never be separately re-judged into a second fault/point.
	ev := judgeRally(&s, []Event{
		{Kind: EvKick, Side: 0, Player: 0},
		{Kind: EvBounceFloor, Side: 1, Pos: Vec3{X: s.Cfg.ServiceLineDist + 1}},
		{Kind: EvHitWall, Side: 1},
	})

	faults := 0
	for _, e := range ev {
		switch e.Kind {
		case EvServeFault:
			faults++
		case EvPoint:
			t.Fatalf("got an EvPoint, want none (this was only the first fault, not a double fault): %+v", ev)
		}
	}
	if faults != 1 {
		t.Fatalf("got %d EvServeFault, want exactly 1 (the trailing EvHitWall must not be re-judged): %+v", faults, ev)
	}
	if s.Phase != PhaseServe {
		t.Errorf("Phase: got %v, want PhaseServe (retrying)", s.Phase)
	}
}
