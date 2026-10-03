package sim

import (
	"math"
	"testing"
)

func TestSanitizeInput(t *testing.T) {
	cfg := DefaultConfig()
	cases := []struct {
		name string
		in   PlayerInput
		side int8
		want PlayerInput
	}{
		{
			name: "NaN collapses MoveX/MoveZ to zero and Target to the rival half's center",
			in: PlayerInput{
				MoveX: math.NaN(), MoveZ: math.NaN(),
				TargetX: math.NaN(), TargetZ: math.NaN(),
			},
			side: 0,
			want: PlayerInput{TargetX: CourtHalfLength / 2},
		},
		{
			name: "axes are clamped to -1..1",
			in:   PlayerInput{MoveX: 5, MoveZ: -5},
			side: 0,
			want: PlayerInput{MoveX: 1, MoveZ: -1, TargetX: cfg.TargetMinFromNet},
		},
		{
			name: "infinities are clamped, not propagated",
			in:   PlayerInput{MoveX: math.Inf(1), MoveZ: math.Inf(-1)},
			side: 0,
			want: PlayerInput{MoveX: 1, MoveZ: -1, TargetX: cfg.TargetMinFromNet},
		},
		{
			name: "unknown button bits are dropped",
			in:   PlayerInput{Buttons: 0xFF},
			side: 0,
			want: PlayerInput{Buttons: btnMask, TargetX: cfg.TargetMinFromNet},
		},
		{
			name: "spin is clamped to {-1,0,1}",
			in:   PlayerInput{Spin: 5},
			side: 0,
			want: PlayerInput{Spin: 1, TargetX: cfg.TargetMinFromNet},
		},
		{
			name: "target is clamped to the rival half for side 0 (positive X)",
			in:   PlayerInput{TargetX: 100, TargetZ: 100},
			side: 0,
			want: PlayerInput{TargetX: CourtHalfLength - cfg.TargetWallMargin, TargetZ: CourtHalfWidth - cfg.TargetWallMargin},
		},
		{
			name: "target is clamped to the rival half for side 1 (negative X)",
			in:   PlayerInput{TargetX: -100, TargetZ: -100},
			side: 1,
			want: PlayerInput{TargetX: -(CourtHalfLength - cfg.TargetWallMargin), TargetZ: -(CourtHalfWidth - cfg.TargetWallMargin)},
		},
		{
			name: "a target on the player's own half is kept (the low-aim shot into the net)",
			in:   PlayerInput{TargetX: -3},
			side: 0,
			want: PlayerInput{TargetX: -3},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := sanitizeInput(c.in, c.side, &cfg)
			if got != c.want {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

// TestEdges_HeldStrikeAcrossMultiStepIsSinglePress checks the v2 contract's
// core promise (docs/design/contact-v2.md §2): a held button produces one
// press edge, no matter how many ticks it stays held.
func TestEdges_HeldStrikeAcrossMultiStepIsSinglePress(t *testing.T) {
	p := &Player{}
	in := PlayerInput{Buttons: BtnStrike}

	if !pressed(p, in, BtnStrike) {
		t.Fatalf("first tick with the button held should be a press edge")
	}
	p.Held = in.Buttons
	for i := 0; i < 10; i++ {
		if pressed(p, in, BtnStrike) {
			t.Fatalf("tick %d: got a press edge again while the button stayed held", i)
		}
	}
}

func TestStepIsNoOpAfterGameOver(t *testing.T) {
	m := NewMatch(DefaultConfig())
	s := m.State()
	s.Phase = PhaseGameOver
	s.Score = [2]int{11, 3}
	m.Restore(s)

	ev := m.Step([MaxPlayers]PlayerInput{{Buttons: BtnStrike, TargetX: 5}, {}})

	if len(ev) != 0 {
		t.Fatalf("got %d events, want 0 after game over", len(ev))
	}
	after := m.State()
	if after.Score != s.Score {
		t.Errorf("Score changed after game over: got %v, want %v", after.Score, s.Score)
	}
}

func TestStepBallFliesDuringPointPause(t *testing.T) {
	m := NewMatch(DefaultConfig())
	s := m.State()
	s.Phase = PhasePoint
	s.Rally.PauseTicks = PointPauseTicks
	s.Ball = Ball{Pos: Vec3{Y: 1}, Vel: Vec3{Y: 2}}
	m.Restore(s)

	m.Step([MaxPlayers]PlayerInput{})

	after := m.State()
	if after.Ball.Pos.Y == 1 {
		t.Error("ball did not move during PhasePoint; it should keep flying visually")
	}
}

// TestStepAllocatesZero enforces the zero-allocations-per-tick contract from
// CLAUDE.md using testing.AllocsPerRun.
func TestStepAllocatesZero(t *testing.T) {
	m := NewMatch(DefaultConfig())
	inputs := [MaxPlayers]PlayerInput{{MoveX: 0.3}, {MoveX: -0.3}}

	allocs := testing.AllocsPerRun(100, func() {
		m.Step(inputs)
	})

	if allocs != 0 {
		t.Errorf("Step allocated %v times per call, want 0", allocs)
	}
}

// TestStepAllocatesZero_2v2 covers the 2v2 path (4 active players) for the
// same zero-alloc contract.
func TestStepAllocatesZero_2v2(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = Mode2v2
	m := NewMatch(cfg)
	inputs := [MaxPlayers]PlayerInput{{MoveX: 0.3}, {MoveX: -0.3}, {MoveZ: 0.2}, {MoveZ: -0.2}}

	allocs := testing.AllocsPerRun(100, func() {
		m.Step(inputs)
	})

	if allocs != 0 {
		t.Errorf("Step allocated %v times per call, want 0", allocs)
	}
}

// TestDeterminism runs two independent matches through a long, identical,
// deterministically-scripted input sequence (via the seeded LCG, never
// math/rand) and checks their resulting states are bit-identical.
func TestDeterminism(t *testing.T) {
	const ticks = 5000

	runMatch := func(seed uint64) State {
		m := NewMatch(DefaultConfig())
		g := newLCG(seed)
		for i := 0; i < ticks; i++ {
			inputs := [MaxPlayers]PlayerInput{scriptedInput(g), scriptedInput(g)}
			m.Step(inputs)
		}
		return m.State()
	}

	a := runMatch(42)
	b := runMatch(42)

	if a != b {
		t.Fatalf("two matches given the same seed diverged:\n a=%+v\n b=%+v", a, b)
	}
}

// TestIntegrationServeToPoint drives a full real-padel serve (docs/design/
// contact-v2.md §7.5: press Strike to drop the toss, wait for the
// mandatory bounce behind the service line, release to strike into the
// receiver's diagonal box) and asserts that an EvPoint eventually gets
// emitted. The press/release is driven off live State, not a hardcoded
// tick count, so it self-corrects (re-tosses) across a let/fault retry.
func TestIntegrationServeToPoint(t *testing.T) {
	m := NewMatch(DefaultConfig())

	const maxTicks = 20 * TickRate
	const targetX, targetZ = 4.0, 1.5 // well inside the box-0 diagonal box (0..ServiceLineDist, Z>=0)

	charging := false
	sawPoint := false
	for i := 0; i < maxTicks; i++ {
		s := m.State()
		in0 := PlayerInput{TargetX: targetX, TargetZ: targetZ}
		switch {
		case s.Phase == PhaseServe && !s.Ball.Tossed:
			charging = true
		case charging && s.Rally.ServeTossBounced:
			charging = false // release now: the toss just bounced legally
		}
		if charging {
			in0.Buttons = BtnStrike
		}

		ev := m.Step([MaxPlayers]PlayerInput{in0})
		for _, e := range ev {
			if e.Kind == EvPoint {
				sawPoint = true
			}
		}
		if sawPoint {
			break
		}
	}

	if !sawPoint {
		t.Fatalf("no EvPoint emitted within %d ticks of a served rally", maxTicks)
	}
}

// TestRestingBallResolvesPoint is a regression test for the "stuck rally"
// bug: a ball that settles on the floor below floorBounceMinSpeed used to
// never emit EvBounceFloor, so a second bounce on the receiving side (which
// should award the kicker the point) could never be judged and the rally
// would hang forever. The resting edge-trigger in resolveFloor fixes this.
func TestRestingBallResolvesPoint(t *testing.T) {
	m := NewMatch(DefaultConfig())
	s := m.State()
	s.Phase = PhaseRally
	s.Rally = RallyState{LastTeam: 0, LastPlayer: 0, BouncesOnSide: [2]int{0, 1}}
	s.Ball = Ball{Pos: Vec3{X: 3, Y: BallRadius + 0.009}, Vel: Vec3{Y: -0.4375}}
	m.Restore(s)

	const maxTicks = 5 * TickRate
	sawPoint := false
	for i := 0; i < maxTicks && !sawPoint; i++ {
		ev := m.Step([MaxPlayers]PlayerInput{})
		for _, e := range ev {
			if e.Kind == EvPoint && e.Side == 0 {
				sawPoint = true
			}
		}
	}

	if !sawPoint {
		t.Fatalf("no EvPoint(side=0) emitted within %d ticks; rally got stuck on a resting ball", maxTicks)
	}
}

// TestRestingBallOnReceiverSideAfterOneBounceAwardsKicker checks the
// resting-edge-trigger directly: a ball that comes to rest on the receiving
// side, after exactly one legal bounce already happened there, must resolve
// the point for the kicker (the receiver failed to return it in time).
func TestRestingBallOnReceiverSideAfterOneBounceAwardsKicker(t *testing.T) {
	s := newRulesTestState()
	s.Rally = RallyState{LastTeam: 0, LastPlayer: 0, BouncesOnSide: [2]int{0, 1}}
	s.Ball = Ball{Pos: Vec3{X: 0.5, Y: BallRadius + 0.0001}, Vel: Vec3{Y: -0.05}}

	ev := stepBall(&s, nil)
	ev = applyRules(&s, ev)

	foundPoint := false
	for _, e := range ev {
		if e.Kind == EvPoint {
			foundPoint = true
			if e.Side != 0 {
				t.Errorf("EvPoint.Side: got %d, want 0 (the kicker)", e.Side)
			}
		}
	}
	if !foundPoint {
		t.Fatalf("got %+v, want an EvPoint once the ball rests on the receiving side", ev)
	}
}

// TestRestingBallOnKickerSideIsAFault mirrors the previous test but for a
// ball resting on the kicker's own side before any legal receiver bounce:
// that is a fault, so the receiver wins the point.
func TestRestingBallOnKickerSideIsAFault(t *testing.T) {
	s := newRulesTestState()
	s.Rally = RallyState{LastTeam: 0, LastPlayer: 0, BouncesOnSide: [2]int{0, 0}}
	s.Ball = Ball{Pos: Vec3{X: -0.5, Y: BallRadius + 0.0001}, Vel: Vec3{Y: -0.05}}

	ev := stepBall(&s, nil)
	ev = applyRules(&s, ev)

	foundPoint := false
	for _, e := range ev {
		if e.Kind == EvPoint {
			foundPoint = true
			if e.Side != 1 {
				t.Errorf("EvPoint.Side: got %d, want 1 (the receiver; kicker's shot never left their side)", e.Side)
			}
		}
	}
	if !foundPoint {
		t.Fatalf("got %+v, want an EvPoint once the ball rests on the kicker's own side", ev)
	}
}

// BenchmarkStep measures the steady-state cost of one simulation tick and
// reports allocations; CLAUDE.md requires zero allocations per tick.
func BenchmarkStep(b *testing.B) {
	m := NewMatch(DefaultConfig())
	inputs := [MaxPlayers]PlayerInput{{MoveX: 0.5, TargetX: 9}, {MoveX: -0.5, TargetX: -9}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m.Step(inputs)
	}
}

// FuzzStep feeds arbitrary (including NaN/Inf) input into Step and asserts
// the simulation never panics and the resulting positions stay finite.
func FuzzStep(f *testing.F) {
	f.Add(0.0, 0.0, uint8(0), 0.0, 0.0, int8(0))
	f.Add(math.NaN(), math.Inf(1), uint8(0xFF), math.Inf(-1), math.NaN(), int8(5))
	f.Add(1.0, 1.0, uint8(BtnStrike|BtnLob), 1.0, 1.0, int8(-5))

	f.Fuzz(func(t *testing.T, moveX, moveZ float64, buttons uint8, targetX, targetZ float64, spin int8) {
		m := NewMatch(DefaultConfig())
		in := PlayerInput{MoveX: moveX, MoveZ: moveZ, Buttons: buttons, TargetX: targetX, TargetZ: targetZ, Spin: spin}

		for i := 0; i < 10; i++ {
			m.Step([MaxPlayers]PlayerInput{in, in})
		}

		s := m.State()
		for _, v := range []float64{
			s.Ball.Pos.X, s.Ball.Pos.Y, s.Ball.Pos.Z,
			s.Players[0].Pos.X, s.Players[0].Pos.Y, s.Players[0].Pos.Z,
			s.Players[1].Pos.X, s.Players[1].Pos.Y, s.Players[1].Pos.Z,
		} {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Fatalf("state has a non-finite value after fuzzed input: %+v", s)
			}
		}
	})
}
