package sim

import "testing"

// --- movement.go: horizontal acceleration / court clamp / inactive slots ---

func TestMovePlayers_DiagonalInputIsNormalized(t *testing.T) {
	s := newPhysicsTestState()
	s.Players[0].OnGround = true
	s.Players[0].Pos = Vec3{X: -5}
	movePlayers(&s, fullInputs(PlayerInput{MoveX: 1, MoveZ: 1}, PlayerInput{}), nil)

	assertFloat(t, "player0.Vel.X == player0.Vel.Z", s.Players[0].Vel.X, s.Players[0].Vel.Z)
	if s.Players[0].Vel.X <= 0 {
		t.Errorf("expected forward progress on the diagonal, got %v", s.Players[0].Vel.X)
	}
}

func TestMovePlayers_Side0ClampedBeforeCrossingNet(t *testing.T) {
	s := newPhysicsTestState()
	s.Players[0].OnGround = true
	s.Players[0].Pos = Vec3{X: -0.4}
	for i := 0; i < 2*TickRate; i++ {
		movePlayers(&s, fullInputs(PlayerInput{MoveX: 1}, PlayerInput{}), nil)
	}
	assertFloat(t, "player0.Pos.X", s.Players[0].Pos.X, -PlayerRadius)
}

func TestMovePlayers_Side1ClampedBeforeCrossingNet(t *testing.T) {
	s := newPhysicsTestState()
	s.Players[1].OnGround = true
	s.Players[1].Pos = Vec3{X: 0.4}
	for i := 0; i < 2*TickRate; i++ {
		movePlayers(&s, fullInputs(PlayerInput{}, PlayerInput{MoveX: -1}), nil)
	}
	assertFloat(t, "player1.Pos.X", s.Players[1].Pos.X, PlayerRadius)
}

func TestMovePlayers_Side0ClampedAtBackWall(t *testing.T) {
	s := newPhysicsTestState()
	s.Players[0].OnGround = true
	s.Players[0].Pos = Vec3{X: -9.6}
	for i := 0; i < 2*TickRate; i++ {
		movePlayers(&s, fullInputs(PlayerInput{MoveX: -1}, PlayerInput{}), nil)
	}
	assertFloat(t, "player0.Pos.X", s.Players[0].Pos.X, -(CourtHalfLength - PlayerRadius))
}

func TestMovePlayers_ClampedAtSideWallOnZ(t *testing.T) {
	s := newPhysicsTestState()
	s.Players[0].OnGround = true
	s.Players[0].Pos = Vec3{X: -5, Z: 4.9}
	for i := 0; i < 2*TickRate; i++ {
		movePlayers(&s, fullInputs(PlayerInput{MoveZ: 1}, PlayerInput{}), nil)
	}
	assertFloat(t, "player0.Pos.Z", s.Players[0].Pos.Z, CourtHalfWidth-PlayerRadius)
}

func TestMovePlayers_OnlyActiveSlotsMove(t *testing.T) {
	s := newPhysicsTestState() // NumPlayers: 2
	s.Players[2].Pos = Vec3{X: -5}
	movePlayers(&s, [MaxPlayers]PlayerInput{{}, {}, {MoveX: 1}, {MoveX: 1}}, nil)

	if s.Players[2].Pos.X != -5 {
		t.Errorf("player2 (inactive slot) moved: got %+v", s.Players[2].Pos)
	}
}

func TestMovePlayers_ZeroAllocs(t *testing.T) {
	s := newPhysicsTestState()
	s.Players[0].OnGround = true
	s.Players[1].OnGround = true
	in := [MaxPlayers]PlayerInput{{MoveX: 0.5}, {MoveX: -0.5}}
	var ev []Event

	allocs := testing.AllocsPerRun(100, func() {
		ev = movePlayers(&s, in, ev[:0])
	})
	if allocs != 0 {
		t.Errorf("movePlayers allocated %v times per call, want 0", allocs)
	}
}

// --- §4/§5: acceleration curve, jump, stamina, charge ----------------------

// TestAccel_ReachesSpeedInExpectedTicks is test plan #4: 6 m/s / 40 m/s^2 =
// 0.15s = 18 ticks at 120 Hz.
func TestAccel_ReachesSpeedInExpectedTicks(t *testing.T) {
	cfg := DefaultConfig()
	wantTicks := 18
	if got := cfg.PlayerSpeed / (cfg.PlayerAccel * Dt); got < float64(wantTicks)-0.01 || got > float64(wantTicks)+0.01 {
		t.Fatalf("sanity: PlayerSpeed/(PlayerAccel*Dt) = %v, want ~%d", got, wantTicks)
	}

	s := newPhysicsTestState()
	s.Players[0].OnGround = true
	in := fullInputs(PlayerInput{MoveX: 1}, PlayerInput{})

	for i := 0; i < wantTicks-1; i++ {
		movePlayers(&s, in, nil)
	}
	if s.Players[0].Vel.X >= cfg.PlayerSpeed {
		t.Fatalf("after %d ticks, velocity already at/above target: %v", wantTicks-1, s.Players[0].Vel.X)
	}

	movePlayers(&s, in, nil)
	assertFloat(t, "player0.Vel.X after the expected number of accel ticks", s.Players[0].Vel.X, cfg.PlayerSpeed)
}

// TestJump_RequiresOnGround is test plan #5 (half): a jump only has an
// effect when OnGround, and gravity still integrates normally otherwise.
func TestJump_RequiresOnGround(t *testing.T) {
	cfg := DefaultConfig()
	s := newPhysicsTestState()
	s.Cfg = cfg
	s.Players[0].OnGround = false
	s.Players[0].Vel.Y = 2
	s.Players[0].Stamina = 1

	ev := movePlayers(&s, fullInputs(PlayerInput{Buttons: BtnJump}, PlayerInput{}), nil)

	for _, e := range ev {
		if e.Kind == EvJump {
			t.Fatalf("got EvJump while airborne, want none")
		}
	}
	wantVelY := 2 - float64(cfg.PlayerGravity*Dt)
	assertFloat(t, "player0.Vel.Y", s.Players[0].Vel.Y, wantVelY)
}

// TestJump_ApexMatchesFormula is test plan #5: apex height must match
// v^2/(2g) within a small tolerance for 120 Hz discretization.
func TestJump_ApexMatchesFormula(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModePlayground
	m := NewMatch(cfg)

	maxY := 0.0
	sawJump := false
	for i := 0; i < 3*TickRate; i++ {
		in := PlayerInput{}
		if i == 0 {
			in.Buttons = BtnJump
		}
		ev := m.Step([MaxPlayers]PlayerInput{in, {}})
		for _, e := range ev {
			if e.Kind == EvJump {
				sawJump = true
				if e.Side != 0 || e.Player != 0 {
					t.Errorf("EvJump: got Side=%d Player=%d, want 0,0", e.Side, e.Player)
				}
			}
		}
		if p := m.State().Players[0]; p.Pos.Y > maxY {
			maxY = p.Pos.Y
		}
	}
	if !sawJump {
		t.Fatalf("no EvJump emitted on the press tick")
	}

	// 120 Hz discretization: the apex tick overshoots the continuous formula
	// by up to ~JumpSpeed*Dt/2 (half a tick's worth of extra climb before
	// gravity catches up), hence the wider tolerance than other float
	// comparisons in this package.
	wantApex := cfg.JumpSpeed * cfg.JumpSpeed / (2 * cfg.PlayerGravity)
	if d := maxY - wantApex; d > 0.03 || d < -0.03 {
		t.Errorf("apex height: got %v, want ~%v (v^2/2g)", maxY, wantApex)
	}
}

// TestJump_AirAccelReduced is test plan #6: horizontal control is weaker
// while airborne.
func TestJump_AirAccelReduced(t *testing.T) {
	cfg := DefaultConfig()
	s := newPhysicsTestState()
	s.Cfg = cfg
	s.Players[0].Pos.Y = 1 // actually airborne, not resting on the floor at Y=0
	s.Players[0].OnGround = false

	movePlayers(&s, fullInputs(PlayerInput{MoveX: 1}, PlayerInput{}), nil)

	wantVelX := float64(cfg.PlayerAirAccel * Dt)
	assertFloat(t, "player0.Vel.X after one airborne tick", s.Players[0].Vel.X, wantVelX)
}

// TestJump_CostsStamina is test plan #6: a jump consumes JumpStaminaCost.
func TestJump_CostsStamina(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModePlayground
	m := NewMatch(cfg)

	m.Step([MaxPlayers]PlayerInput{{Buttons: BtnJump}, {}})

	got := m.State().Players[0].Stamina
	want := 1 - cfg.JumpStaminaCost + cfg.StaminaRegen*Dt // regen also runs this same tick
	assertFloat(t, "stamina after one jump", got, want)
}

// TestJump_BlockedWhenInsufficientStamina is test plan #6.
func TestJump_BlockedWhenInsufficientStamina(t *testing.T) {
	cfg := DefaultConfig()
	s := newPhysicsTestState()
	s.Cfg = cfg
	s.Players[0].OnGround = true
	s.Players[0].Stamina = cfg.JumpStaminaCost / 2

	ev := movePlayers(&s, fullInputs(PlayerInput{Buttons: BtnJump}, PlayerInput{}), nil)

	for _, e := range ev {
		if e.Kind == EvJump {
			t.Fatalf("got EvJump with insufficient stamina")
		}
	}
	if !s.Players[0].OnGround {
		t.Errorf("OnGround: got false, want true (the jump should have been blocked)")
	}
}

// TestStamina_DrainsOnlyWhileSprintingAndMoving is test plan #7.
func TestStamina_DrainsOnlyWhileSprintingAndMoving(t *testing.T) {
	cfg := DefaultConfig()
	s := newPhysicsTestState()
	s.Cfg = cfg
	s.Players[0].OnGround = true
	s.Players[0].Stamina = 1

	movePlayers(&s, fullInputs(PlayerInput{Buttons: BtnSprint}, PlayerInput{}), nil)
	assertFloat(t, "stamina, sprint held but standing still", s.Players[0].Stamina, 1)

	movePlayers(&s, fullInputs(PlayerInput{Buttons: BtnSprint, MoveX: 1}, PlayerInput{}), nil)
	want := 1 - float64(cfg.StaminaDrain*Dt)
	assertFloat(t, "stamina after one sprinting+moving tick", s.Players[0].Stamina, want)
}

// TestStamina_DepletionTriggersLockThenRegen is test plan #8:
// ticksOf(0.75) = 90 ticks with no regen after hitting 0.
func TestStamina_DepletionTriggersLockThenRegen(t *testing.T) {
	cfg := DefaultConfig()
	s := newPhysicsTestState()
	s.Cfg = cfg
	s.Players[0].OnGround = true
	s.Players[0].Stamina = float64(cfg.StaminaDrain * Dt) // exactly one drain tick from zero
	sprint := fullInputs(PlayerInput{Buttons: BtnSprint, MoveX: 1}, PlayerInput{})

	movePlayers(&s, sprint, nil)
	if s.Players[0].Stamina != 0 {
		t.Fatalf("stamina: got %v, want exactly 0", s.Players[0].Stamina)
	}
	wantLockTicks := ticksOf(cfg.StaminaLockTime)
	if int(s.Players[0].StaminaLock) != wantLockTicks {
		t.Fatalf("StaminaLock: got %d, want %d", s.Players[0].StaminaLock, wantLockTicks)
	}

	idle := fullInputs(PlayerInput{}, PlayerInput{})
	for i := 0; i < wantLockTicks; i++ {
		movePlayers(&s, idle, nil)
		if s.Players[0].Stamina != 0 {
			t.Fatalf("tick %d: stamina regenerated before the lock expired: %v", i, s.Players[0].Stamina)
		}
	}

	movePlayers(&s, idle, nil)
	want := float64(cfg.StaminaRegen * Dt)
	assertFloat(t, "stamina, first regen tick once the lock expires", s.Players[0].Stamina, want)
}

// TestCharge_SaturatesAtMax_SlowsMovement is test plan #10: a saturated
// charge (108 ticks) caps movement speed at ChargeMoveFactor (0.8x).
func TestCharge_SaturatesAtMax_SlowsMovement(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.chargeMaxTicks != 108 {
		t.Fatalf("sanity: chargeMaxTicks got %d, want 108", cfg.chargeMaxTicks)
	}
	s := newPhysicsTestState()
	s.Cfg = cfg
	s.Players[0].OnGround = true
	s.Players[0].ChargeTicks = uint16(cfg.chargeMaxTicks)
	in := fullInputs(PlayerInput{MoveX: 1}, PlayerInput{})

	for i := 0; i < 60; i++ {
		movePlayers(&s, in, nil)
	}

	want := cfg.PlayerSpeed * cfg.ChargeMoveFactor
	assertFloat(t, "player0.Vel.X plateau while charging", s.Players[0].Vel.X, want)
	if s.Players[0].Vel.X >= cfg.PlayerSpeed {
		t.Errorf("charging should slow movement below PlayerSpeed: got %v", s.Players[0].Vel.X)
	}
}

// --- §19.4: same-team capsule separation (2v2) -----------------------------

func TestSeparateTeammates_PushesOverlapApart(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = Mode2v2
	s := State{Cfg: cfg, NumPlayers: 4}
	s.Players[0] = Player{Pos: Vec3{X: -5, Z: 0.1}, Team: 0, OnGround: true}
	s.Players[1] = Player{Pos: Vec3{X: -5, Z: -0.1}, Team: 0, OnGround: true}
	s.Players[2] = Player{Pos: Vec3{X: 5}, Team: 1, OnGround: true}
	s.Players[3] = Player{Pos: Vec3{X: 5, Z: 3}, Team: 1, OnGround: true}

	movePlayers(&s, [MaxPlayers]PlayerInput{}, nil)

	dist := s.Players[1].Pos.Sub(s.Players[0].Pos).Len()
	if dist < 2*PlayerRadius-epsilon {
		t.Fatalf("teammates still overlapping: dist=%v, want >= %v", dist, 2*PlayerRadius)
	}
}

func TestSeparateTeammates_NeverMovesOpponentsApart(t *testing.T) {
	s := newPhysicsTestState() // NumPlayers: 2, no separation pass runs at all
	s.Players[0].OnGround = true
	s.Players[1].OnGround = true
	s.Players[0].Pos = Vec3{X: -1}
	s.Players[1].Pos = Vec3{X: 1}

	movePlayers(&s, [MaxPlayers]PlayerInput{}, nil)

	assertFloat(t, "player0.Pos.X unaffected by opponent proximity", s.Players[0].Pos.X, -1)
}

// TestSeparateTeammates_DegenerateTieBreaksDeterministically covers the
// dist==0 fallback: two exactly-coincident teammates must still separate,
// and always in the same direction (break the tie on +Z), not depend on
// floating point luck.
func TestSeparateTeammates_DegenerateTieBreaksDeterministically(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = Mode2v2
	s := State{Cfg: cfg, NumPlayers: 4}
	s.Players[0] = Player{Pos: Vec3{X: -5}, Team: 0, OnGround: true}
	s.Players[1] = Player{Pos: Vec3{X: -5}, Team: 0, OnGround: true}
	s.Players[2] = Player{Pos: Vec3{X: 5}, Team: 1, OnGround: true}
	s.Players[3] = Player{Pos: Vec3{X: 5}, Team: 1, OnGround: true}

	movePlayers(&s, [MaxPlayers]PlayerInput{}, nil)

	if s.Players[0].Pos == s.Players[1].Pos {
		t.Fatalf("coincident teammates were not separated")
	}
	if s.Players[0].Pos.Z >= s.Players[1].Pos.Z {
		t.Errorf("expected player0 pushed to -Z and player1 to +Z deterministically: got %+v / %+v",
			s.Players[0].Pos, s.Players[1].Pos)
	}
}

// TestDeterminism_2v2 mirrors TestDeterminism but exercises the 2v2 path
// (separation + 4 active players) for the same bit-identical-replay
// contract.
func TestDeterminism_2v2(t *testing.T) {
	const ticks = 3000

	runMatch := func(seed uint64) State {
		cfg := DefaultConfig()
		cfg.Mode = Mode2v2
		m := NewMatch(cfg)
		g := newLCG(seed)
		for i := 0; i < ticks; i++ {
			inputs := [MaxPlayers]PlayerInput{
				scriptedInput(g), scriptedInput(g), scriptedInput(g), scriptedInput(g),
			}
			m.Step(inputs)
		}
		return m.State()
	}

	a := runMatch(7)
	b := runMatch(7)
	if a != b {
		t.Fatalf("two 2v2 matches given the same seed diverged:\n a=%+v\n b=%+v", a, b)
	}
}
