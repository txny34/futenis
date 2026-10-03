package sim

import "testing"

// strikeRelease returns the two-tick input sequence that presses then
// releases a Strike/Lob, per the v2 held-levels contract (docs/design/
// contact-v2.md §2): tick0 holds the button, tick1 releases it, which is
// when the wave-0 compat shim (applyKicks/performKick) actually fires.
func strikeRelease(bit uint8, targetX, targetZ float64) (press, release PlayerInput) {
	press = PlayerInput{Buttons: bit, TargetX: targetX, TargetZ: targetZ}
	release = PlayerInput{TargetX: targetX, TargetZ: targetZ}
	return
}

func TestApplyKicks(t *testing.T) {
	t.Run("non-server cannot kick during serve", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Phase = PhaseServe
		s.Server = 0
		s.Rally.ServerPlayer = 0
		s.Players[1].Pos = Vec3{X: 0.5}
		s.Ball.Pos = Vec3{X: 0.5}

		press, release := strikeRelease(BtnStrike, -9, 0)
		s.Players[1].Held = press.Buttons
		ev := applyKicks(&s, fullInputs(PlayerInput{}, release), nil)

		if len(ev) != 0 {
			t.Fatalf("got %d events, want 0", len(ev))
		}
		if s.Phase != PhaseServe {
			t.Errorf("Phase: got %v, want PhaseServe", s.Phase)
		}
	})

	t.Run("server kick during serve switches to rally and emits EvKick", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Phase = PhaseServe
		s.Server = 0
		s.Rally.ServerPlayer = 0
		s.Players[0].Pos = Vec3{X: -0.5}
		s.Ball.Pos = Vec3{X: -0.5}
		s.Players[0].Held = BtnStrike

		ev := applyKicks(&s, fullInputs(PlayerInput{TargetX: 9}, PlayerInput{}), nil)

		if s.Phase != PhaseRally {
			t.Fatalf("Phase: got %v, want PhaseRally", s.Phase)
		}
		if len(ev) != 1 || ev[0].Kind != EvKick || ev[0].Side != 0 || ev[0].Player != 0 {
			t.Fatalf("got %+v, want one EvKick with Side 0, Player 0", ev)
		}
		if s.Ball.Vel.Len() == 0 {
			t.Error("ball velocity was not set by the kick")
		}
	})

	t.Run("out of reach does not kick", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Phase = PhaseRally
		s.Rally.LastTeam = 1
		s.Players[0].Pos = Vec3{X: -5}
		s.Players[0].Held = BtnStrike
		s.Ball.Pos = Vec3{X: 0}

		ev := applyKicks(&s, fullInputs(PlayerInput{TargetX: 9}, PlayerInput{}), nil)

		if len(ev) != 0 {
			t.Fatalf("got %d events, want 0", len(ev))
		}
	})

	t.Run("either side may kick during rally", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Phase = PhaseRally
		s.Players[1].Pos = Vec3{X: 0.5}
		s.Players[1].Held = BtnStrike
		s.Ball.Pos = Vec3{X: 0.5}

		ev := applyKicks(&s, fullInputs(PlayerInput{}, PlayerInput{TargetX: -9}), nil)

		if len(ev) != 1 || ev[0].Kind != EvKick || ev[0].Side != 1 {
			t.Fatalf("got %+v, want one EvKick with Side 1", ev)
		}
	})

	t.Run("simultaneous release: side 0 wins the tie-break", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Phase = PhaseRally
		s.Players[0].Pos = Vec3{X: -0.35}
		s.Players[0].Held = BtnStrike
		s.Players[1].Pos = Vec3{X: 0.35}
		s.Players[1].Held = BtnStrike
		s.Ball.Pos = Vec3{X: 0}

		ev := applyKicks(&s, fullInputs(PlayerInput{TargetX: 9}, PlayerInput{TargetX: -9}), nil)

		if len(ev) != 1 {
			t.Fatalf("got %d events, want exactly 1 (only one kick per tick)", len(ev))
		}
		if ev[0].Side != 0 {
			t.Errorf("Side: got %d, want 0", ev[0].Side)
		}
	})

	t.Run("a pressed (not yet released) button does not kick", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Phase = PhaseRally
		s.Players[0].Pos = Vec3{X: -0.5}
		s.Ball.Pos = Vec3{X: -0.5}

		ev := applyKicks(&s, fullInputs(PlayerInput{Buttons: BtnStrike, TargetX: 9}, PlayerInput{}), nil)

		if len(ev) != 0 {
			t.Fatalf("got %d events, want 0 (press edge alone must not fire a kick)", len(ev))
		}
		if s.Players[0].Held != BtnStrike {
			t.Errorf("Held: got %b, want BtnStrike recorded for next tick's edge detection", s.Players[0].Held)
		}
	})
}
