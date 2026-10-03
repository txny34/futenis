package sim

import "testing"

// --- enforceServeClock (unit) ------------------------------------------

// TestEnforceServeClock_TossesAfterLimit exercises enforceServeClock
// directly: an idle server (never presses Strike) must have the ball
// auto-dropped once Rally.ServeWaitTicks reaches cfg.serveTimeLimitTicks,
// exactly once, not before.
func TestEnforceServeClock_TossesAfterLimit(t *testing.T) {
	s := newServeTestState(0)
	s.Cfg.ServeTimeLimit = 1.0
	s.Cfg.Sanitize()
	limit := s.Cfg.serveTimeLimitTicks

	for i := 0; i < limit-1; i++ {
		ev := enforceServeClock(&s, nil)
		if len(ev) != 0 {
			t.Fatalf("tick %d: got %+v, want no toss yet (limit=%d)", i, ev, limit)
		}
		if s.Ball.Tossed {
			t.Fatalf("tick %d: ball tossed early", i)
		}
	}

	ev := enforceServeClock(&s, nil)
	if len(ev) != 1 || ev[0].Kind != EvToss {
		t.Fatalf("got %+v, want exactly one EvToss once the clock expires", ev)
	}
	if !s.Ball.Tossed {
		t.Fatalf("Ball.Tossed: got false, want true after the auto-toss")
	}
}

// TestEnforceServeClock_ResetsOnceTossed ensures the clock does not keep
// firing (or re-tossing) once the ball has actually been released, whether
// by the player or by the clock itself.
func TestEnforceServeClock_ResetsOnceTossed(t *testing.T) {
	s := newServeTestState(0)
	s.Rally.ServeWaitTicks = 50
	s.Ball.Tossed = true

	ev := enforceServeClock(&s, nil)

	if len(ev) != 0 {
		t.Fatalf("got %+v, want no event once the ball is already tossed", ev)
	}
	if s.Rally.ServeWaitTicks != 0 {
		t.Errorf("ServeWaitTicks: got %d, want reset to 0", s.Rally.ServeWaitTicks)
	}
}

// TestEnforceServeClock_NoOpOutsidePhaseServe guards against the clock
// firing a stray toss once a fault/point has already moved the match out of
// PhaseServe within the same tick's event batch.
func TestEnforceServeClock_NoOpOutsidePhaseServe(t *testing.T) {
	s := newServeTestState(0)
	s.Rally.ServeWaitTicks = 999999
	s.Phase = PhasePoint

	ev := enforceServeClock(&s, nil)

	if len(ev) != 0 {
		t.Fatalf("got %+v, want no event outside PhaseServe", ev)
	}
}

// --- idle server, end to end via Match.Step -----------------------------

// TestMatch_IdleServerAutoTossesAndPlayContinues is the full-stack version
// of the "serve can hang forever" fix: a server that never presses Strike
// must still see the ball drop on its own (EvToss) well before
// ServeTimeLimit, and the match must keep progressing (the serve's mandatory
// toss bounce eventually gets judged) instead of sitting in PhaseServe
// forever.
func TestMatch_IdleServerAutoTossesAndPlayContinues(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ServeTimeLimit = 0.5 // short, so the test does not need 8s of ticks
	cfg.Sanitize()
	m := NewMatch(cfg)

	var idle [MaxPlayers]PlayerInput
	tossed := false
	limitTicks := ticksOf(0.5)
	for i := 0; i < limitTicks+5; i++ {
		ev := m.Step(idle)
		for _, e := range ev {
			if e.Kind == EvToss {
				tossed = true
			}
		}
		if tossed {
			break
		}
	}

	if !tossed {
		t.Fatalf("server never pressed Strike, and the serve clock never auto-tossed within %d ticks", limitTicks+5)
	}
	if !m.State().Ball.Tossed {
		t.Fatalf("Ball.Tossed: got false after the auto-toss event")
	}

	// The ball is now falling under real physics while still idle; it must
	// eventually bounce and get judged (fault, since nobody ever strikes
	// it) instead of the match getting stuck in PhaseServe forever.
	judged := false
	for i := 0; i < TickRate*5; i++ {
		ev := m.Step(idle)
		for _, e := range ev {
			if e.Kind == EvServeFault || e.Kind == EvPoint {
				judged = true
			}
		}
		if judged {
			break
		}
	}
	if !judged {
		t.Fatalf("idle server after the auto-toss: match never resolved the point (stuck)")
	}
}
