package sim

import "testing"

// newStrikeTestState is a Phase==PhaseRally 2-player state (so canKick never
// depends on which player is the server), with the ball far from both
// players by default -- tests move it into reach explicitly.
func newStrikeTestState() State {
	s := State{Cfg: DefaultConfig(), NumPlayers: 2}
	s.Players[0] = Player{Team: 0, OnGround: true, Pos: Vec3{X: -5}}
	s.Players[1] = Player{Team: 1, OnGround: true, Pos: Vec3{X: 5}}
	s.Phase = PhaseRally
	s.Rally.LastTeam = -1
	s.Rally.LastPlayer = -1
	s.Server = 0
	return s
}

// --- updateCharge ----------------------------------------------------------

func TestUpdateCharge_PressLatchesShotTypeAndStartsCharge(t *testing.T) {
	s := newStrikeTestState()
	p := &s.Players[0]

	updateCharge(&s, 0, PlayerInput{Buttons: BtnLob}, nil)

	if p.ShotType != ShotLob {
		t.Errorf("ShotType: got %v, want ShotLob", p.ShotType)
	}
	if p.ChargeTicks != 1 {
		t.Errorf("ChargeTicks: got %d, want 1 (one tick held since press)", p.ChargeTicks)
	}
	if p.Held != BtnLob {
		t.Errorf("Held: got %b, want BtnLob", p.Held)
	}
}

func TestUpdateCharge_ChargeSaturatesAtMax(t *testing.T) {
	s := newStrikeTestState()
	p := &s.Players[0]
	max := s.Cfg.chargeMaxTicks

	for i := 0; i < max+20; i++ {
		updateCharge(&s, 0, PlayerInput{Buttons: BtnStrike}, nil)
	}

	if int(p.ChargeTicks) != max {
		t.Errorf("ChargeTicks: got %d, want saturated at %d", p.ChargeTicks, max)
	}
}

func TestUpdateCharge_ReleaseFreezesBuffer(t *testing.T) {
	s := newStrikeTestState()
	p := &s.Players[0]

	updateCharge(&s, 0, PlayerInput{Buttons: BtnStrike, TargetX: 7, TargetZ: -2, Spin: 1}, nil)
	for i := 0; i < 5; i++ {
		updateCharge(&s, 0, PlayerInput{Buttons: BtnStrike, TargetX: 7, TargetZ: -2, Spin: 1}, nil)
	}
	updateCharge(&s, 0, PlayerInput{TargetX: 7, TargetZ: -2, Spin: 1}, nil) // release

	if p.Buffer == 0 {
		t.Fatal("Buffer: got 0, want frozen at cfg.bufTicks after release")
	}
	wantCharge := 6.0 / float64(s.Cfg.chargeMaxTicks)
	assertFloat(t, "BufCharge", p.BufCharge, wantCharge)
	if p.BufTarget.X != 7 || p.BufTarget.Z != -2 {
		t.Errorf("BufTarget: got %+v, want {7, BallRadius, -2}", p.BufTarget)
	}
	assertFloat(t, "BufTarget.Y", p.BufTarget.Y, BallRadius)
	if p.BufSpin != 1 {
		t.Errorf("BufSpin: got %d, want 1", p.BufSpin)
	}
	if p.ChargeTicks != 0 {
		t.Errorf("ChargeTicks: got %d, want reset to 0 after release", p.ChargeTicks)
	}
}

func TestUpdateCharge_RepressCancelsOldBuffer(t *testing.T) {
	s := newStrikeTestState()
	p := &s.Players[0]

	updateCharge(&s, 0, PlayerInput{Buttons: BtnStrike}, nil)
	updateCharge(&s, 0, PlayerInput{}, nil) // release -> buffered
	if p.Buffer == 0 {
		t.Fatal("setup: expected a buffered intention after release")
	}

	updateCharge(&s, 0, PlayerInput{Buttons: BtnLob}, nil) // re-press

	if p.Buffer != 0 {
		t.Errorf("Buffer: got %d, want 0 (re-press cancels the old buffer)", p.Buffer)
	}
	if p.ShotType != ShotLob {
		t.Errorf("ShotType: got %v, want ShotLob (the new press)", p.ShotType)
	}
}

// --- applyStrikes: buffer timing --------------------------------------------

func TestApplyStrikes_StrikesWithinBufferWindow(t *testing.T) {
	s := newStrikeTestState()
	ps := &s.Cfg.Parts[PartFoot]
	s.Players[0].Pos = Vec3{X: 0}
	s.Ball.Pos = Vec3{X: 0, Y: ps.HIdeal, Z: 0}
	s.Players[0].Buffer = uint8(s.Cfg.bufTicks)
	s.Players[0].BufCharge = 1
	s.Players[0].BufTarget = Vec3{X: 9, Y: BallRadius, Z: 0}
	s.Players[0].ShotType = ShotDrive

	ev := applyStrikes(&s, fullInputs(PlayerInput{}, PlayerInput{}), nil)

	if len(ev) != 1 || ev[0].Kind != EvKick {
		t.Fatalf("got %+v, want exactly one EvKick (ball already at the ideal contact point)", ev)
	}
	if s.Players[0].Buffer != 0 {
		t.Errorf("Buffer: got %d, want 0 after striking", s.Players[0].Buffer)
	}
	if s.Ball.Vel.Len() == 0 {
		t.Error("ball velocity was not set by the strike")
	}
}

func TestApplyStrikes_ExpiresEmitsWhiff(t *testing.T) {
	s := newStrikeTestState()
	s.Players[0].Pos = Vec3{X: 0}
	s.Ball.Pos = Vec3{X: 100, Y: 0.2} // nowhere near reach
	s.Players[0].Buffer = 3

	var ev []Event
	for i := 0; i < 3; i++ {
		ev = applyStrikes(&s, fullInputs(PlayerInput{}, PlayerInput{}), nil)
	}

	if len(ev) != 1 || ev[0].Kind != EvWhiff {
		t.Fatalf("got %+v, want exactly one EvWhiff on the buffer's last tick", ev)
	}
	if ev[0].Side != 0 || ev[0].Player != 0 {
		t.Errorf("EvWhiff: got Side=%d Player=%d, want Side=0 Player=0", ev[0].Side, ev[0].Player)
	}
}

func TestApplyStrikes_WaitsForBetterTick(t *testing.T) {
	s := newStrikeTestState()
	s.Cfg.StrikeBufferTime = 1.0 // generous window so the fall has time to arrive
	s.Cfg.Sanitize()

	s.Players[0].Pos = Vec3{X: 0}
	s.Ball.Pos = Vec3{X: 0, Y: 0.90} // falls from above the knee's ideal height (0.60)
	s.Players[0].Buffer = uint8(s.Cfg.bufTicks)
	s.Players[0].BufCharge = 1
	s.Players[0].BufTarget = Vec3{X: 9, Y: BallRadius, Z: 0}
	s.Players[0].ShotType = ShotDrive

	kickTick := -1
	for tick := 0; tick < s.Cfg.bufTicks; tick++ {
		s.Tick = uint32(tick)
		ev := applyStrikes(&s, fullInputs(PlayerInput{}, PlayerInput{}), nil)
		for _, e := range ev {
			if e.Kind == EvKick {
				kickTick = tick
			}
		}
		if kickTick >= 0 {
			break
		}
		stepBall(&s, nil)
	}

	if kickTick < 0 {
		t.Fatal("ball never got struck within the buffer window")
	}
	if kickTick == 0 {
		t.Error("struck on the very first tick despite the contact error still improving (ball far above the ideal height)")
	}
}

// --- bestPartConstrained gates, exercised through applyStrikes -------------

func TestBicycle_RequiresAirborneAndFullCharge(t *testing.T) {
	s := newStrikeTestState()
	ps := &s.Cfg.Parts[PartBicycle]
	s.Players[0].Pos = Vec3{X: 0}
	s.Players[0].OnGround = true // grounded: bicycle must not be offered
	s.Ball.Pos = Vec3{X: 0, Y: ps.HIdeal, Z: 0}
	s.Players[0].Buffer = 1
	s.Players[0].BufCharge = 1
	s.Players[0].BufTarget = Vec3{X: 9, Y: BallRadius}

	ev := applyStrikes(&s, fullInputs(PlayerInput{}, PlayerInput{}), nil)

	for _, e := range ev {
		if e.Kind == EvKick && e.Part == PartBicycle {
			t.Fatalf("bicycle contact resolved while OnGround: %+v", e)
		}
	}
}

func TestLobDowngradesToDriveOnShoulder(t *testing.T) {
	s := newStrikeTestState()
	ps := &s.Cfg.Parts[PartShoulder]
	if ps.AllowLob {
		t.Fatal("test assumption broken: PartShoulder.AllowLob should be false")
	}
	s.Players[0].Pos = Vec3{X: 0}
	s.Ball.Pos = Vec3{X: 0, Y: ps.HIdeal, Z: 0}
	s.Players[0].Buffer = 1
	s.Players[0].BufCharge = 1
	s.Players[0].BufTarget = Vec3{X: 9, Y: BallRadius}
	s.Players[0].ShotType = ShotLob

	h := strikeHash(s.Tick, s.Players[0].Team, s.Ball.Pos)
	tgt := deviateTarget(&s.Cfg, s.Players[0].BufTarget, s.Ball.Pos, ps, 0, h)
	speed := shotSpeed(ps, 1, 0, h, &s.Cfg)
	speed += momentumBonus(s.Players[0].Vel, s.Ball.Pos, tgt, &s.Cfg)
	want := solveLaunch(&s.Cfg, s.Ball.Pos, tgt, ShotDrive, speed, 1)
	want = clampLen(want, ps.SpeedMax+s.Cfg.MomentumMax)

	ev := applyStrikes(&s, fullInputs(PlayerInput{}, PlayerInput{}), nil)

	if len(ev) != 1 || ev[0].Kind != EvKick || ev[0].Part != PartShoulder {
		t.Fatalf("got %+v, want one EvKick on PartShoulder", ev)
	}
	if s.Ball.Vel != want {
		t.Errorf("Vel: got %+v, want %+v (Lob downgraded to Drive math since the shoulder disallows lob)", s.Ball.Vel, want)
	}
}

func TestSpinClampedPerPart(t *testing.T) {
	s := newStrikeTestState()
	ps := &s.Cfg.Parts[PartChest] // SpinMin/Max == 0: "no se corta con el pecho"
	s.Players[0].Pos = Vec3{X: 0}
	s.Ball.Pos = Vec3{X: 0, Y: ps.HIdeal, Z: 0}
	s.Players[0].Buffer = 1
	s.Players[0].BufCharge = 1
	s.Players[0].BufTarget = Vec3{X: 9, Y: BallRadius}
	s.Players[0].BufSpin = 1

	applyStrikes(&s, fullInputs(PlayerInput{}, PlayerInput{}), nil)

	assertFloat(t, "Ball.Spin", s.Ball.Spin, 0)
}

func TestServe_RestrictsToFootKnee(t *testing.T) {
	s := newStrikeTestState()
	s.Phase = PhaseServe
	s.Rally.ServerPlayer = 0
	ps := &s.Cfg.Parts[PartChest]
	s.Players[0].Pos = Vec3{X: 0}
	s.Ball.Pos = Vec3{X: 0, Y: ps.HIdeal, Z: 0} // chest height: legal in a rally, not on serve
	s.Players[0].Buffer = 1
	s.Players[0].BufCharge = 1
	s.Players[0].BufTarget = Vec3{X: 9, Y: BallRadius}

	ev := applyStrikes(&s, fullInputs(PlayerInput{}, PlayerInput{}), nil)

	for _, e := range ev {
		if e.Kind == EvKick {
			t.Fatalf("got a serve contact at chest height, want none (serve restricted to foot/knee): %+v", e)
		}
	}
}

// --- pickStriker / simultaneous strike tie-break (§7.3) ---------------------

func TestSimultaneousStrike_LowerErrorWins(t *testing.T) {
	s := newStrikeTestState()
	cand := [MaxPlayers]candidate{
		{ok: true, part: PartFoot, u: 0.5},
		{ok: true, part: PartFoot, u: 0.1},
	}
	if got := pickStriker(&s, &cand); got != 1 {
		t.Errorf("got %d, want 1 (lower error)", got)
	}
}

func TestSimultaneousStrike_TieGoesToNonLastToucher(t *testing.T) {
	s := newStrikeTestState()
	s.Rally.LastTeam = 0 // team 0 touched last
	cand := [MaxPlayers]candidate{
		{ok: true, part: PartFoot, u: 0.3},
		{ok: true, part: PartFoot, u: 0.3},
	}
	if got := pickStriker(&s, &cand); got != 1 {
		t.Errorf("got %d, want 1 (team 1 did not touch last)", got)
	}
}

func TestSimultaneousStrike_NoTouchYetGoesToServer(t *testing.T) {
	s := newStrikeTestState()
	s.Rally.LastTeam = -1
	s.Server = 1
	cand := [MaxPlayers]candidate{
		{ok: true, part: PartFoot, u: 0.3},
		{ok: true, part: PartFoot, u: 0.3},
	}
	if got := pickStriker(&s, &cand); got != 1 {
		t.Errorf("got %d, want 1 (the server)", got)
	}
}

func TestPickStriker_NoneWhenNoCandidateIsOk(t *testing.T) {
	s := newStrikeTestState()
	var cand [MaxPlayers]candidate
	if got := pickStriker(&s, &cand); got != -1 {
		t.Errorf("got %d, want -1", got)
	}
}

// --- EvKick payload ----------------------------------------------------------

func TestKickEvent_CarriesPartQualityVelSpin(t *testing.T) {
	s := newStrikeTestState()
	ps := &s.Cfg.Parts[PartFoot]
	s.Players[0].Pos = Vec3{X: 0}
	s.Ball.Pos = Vec3{X: 0, Y: ps.HIdeal, Z: 0}
	s.Players[0].Buffer = 1
	s.Players[0].BufCharge = 1
	s.Players[0].BufTarget = Vec3{X: 9, Y: BallRadius}
	s.Players[0].BufSpin = 1

	ev := applyStrikes(&s, fullInputs(PlayerInput{}, PlayerInput{}), nil)

	if len(ev) != 1 || ev[0].Kind != EvKick {
		t.Fatalf("got %+v, want one EvKick", ev)
	}
	e := ev[0]
	if e.Part != PartFoot {
		t.Errorf("Part: got %v, want PartFoot", e.Part)
	}
	if e.Quality != QualPerfect {
		t.Errorf("Quality: got %v, want QualPerfect (ball exactly at HIdeal, dH=0)", e.Quality)
	}
	if e.Vel != s.Ball.Vel {
		t.Errorf("Vel: got %+v, want %+v (the post-contact ball velocity)", e.Vel, s.Ball.Vel)
	}
	if e.Spin != s.Ball.Spin {
		t.Errorf("Spin: got %v, want %v", e.Spin, s.Ball.Spin)
	}
}

// --- zero allocations --------------------------------------------------------

func TestApplyStrikes_ZeroAllocs(t *testing.T) {
	s := newStrikeTestState()
	s.Players[0].Pos = Vec3{X: -5}
	s.Ball.Pos = Vec3{X: 100} // never in reach: exercises updateCharge + the miss path only
	ev := make([]Event, 0, 8)
	in := fullInputs(PlayerInput{}, PlayerInput{})

	allocs := testing.AllocsPerRun(100, func() {
		ev = applyStrikes(&s, in, ev[:0])
	})
	if allocs != 0 {
		t.Errorf("applyStrikes allocated %v times per run, want 0", allocs)
	}
}
