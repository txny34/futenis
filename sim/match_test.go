package sim

import (
	"math"
	"testing"
)

func TestSanitizeInput(t *testing.T) {
	cases := []struct {
		name string
		in   PlayerInput
		want PlayerInput
	}{
		{
			name: "NaN collapses to zero on every float field",
			in: PlayerInput{
				MoveX: math.NaN(), MoveZ: math.NaN(),
				AimX: math.NaN(), AimZ: math.NaN(),
				Loft: math.NaN(), Power: math.NaN(),
			},
			want: PlayerInput{},
		},
		{
			name: "axes are clamped to -1..1",
			in:   PlayerInput{MoveX: 5, MoveZ: -5, AimX: 10, AimZ: -10},
			want: PlayerInput{MoveX: 1, MoveZ: -1, AimX: 1, AimZ: -1},
		},
		{
			name: "loft/power are clamped to 0..1",
			in:   PlayerInput{Loft: -5, Power: 5},
			want: PlayerInput{Loft: 0, Power: 1},
		},
		{
			name: "infinities are clamped, not propagated",
			in:   PlayerInput{MoveX: math.Inf(1), MoveZ: math.Inf(-1)},
			want: PlayerInput{MoveX: 1, MoveZ: -1},
		},
		{
			name: "kick flag passes through untouched",
			in:   PlayerInput{Kick: true},
			want: PlayerInput{Kick: true},
		},
		{
			name: "already-valid input is unchanged",
			in:   PlayerInput{MoveX: 0.3, MoveZ: -0.4, AimX: 0.1, AimZ: -0.1, Loft: 0.5, Power: 0.7},
			want: PlayerInput{MoveX: 0.3, MoveZ: -0.4, AimX: 0.1, AimZ: -0.1, Loft: 0.5, Power: 0.7},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := sanitizeInput(c.in)
			if got != c.want {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestStepIsNoOpAfterGameOver(t *testing.T) {
	m := NewMatch(DefaultConfig())
	s := m.State()
	s.Phase = PhaseGameOver
	s.Score = [2]int{11, 3}
	m.Restore(s)

	ev := m.Step([2]PlayerInput{{Kick: true, AimX: 1, Power: 1}, {}})

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

	m.Step([2]PlayerInput{{}, {}})

	after := m.State()
	if after.Ball.Pos.Y == 1 {
		t.Error("ball did not move during PhasePoint; it should keep flying visually")
	}
}

// TestStepAllocatesZero enforces the zero-allocations-per-tick contract from
// CLAUDE.md using testing.AllocsPerRun.
func TestStepAllocatesZero(t *testing.T) {
	m := NewMatch(DefaultConfig())
	inputs := [2]PlayerInput{{MoveX: 0.3}, {MoveX: -0.3}}

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
			inputs := [2]PlayerInput{scriptedInput(g), scriptedInput(g)}
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

// TestIntegrationServeToPoint drives a full serve with a strong, straight,
// well-lofted kick and asserts that an EvPoint eventually gets emitted
// before the ball returns to a serve.
func TestIntegrationServeToPoint(t *testing.T) {
	m := NewMatch(DefaultConfig())

	serverInput := PlayerInput{Kick: true, AimX: 1, Loft: 0.3, Power: 1}
	const maxTicks = 10 * TickRate

	sawPoint := false
	for i := 0; i < maxTicks; i++ {
		var in [2]PlayerInput
		if i == 0 {
			in[0] = serverInput
		}
		ev := m.Step(in)
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
	s.Rally = RallyState{LastKicker: 0, BouncesOnSide: [2]int{0, 1}}
	s.Ball = Ball{Pos: Vec3{X: 3, Y: BallRadius + 0.009}, Vel: Vec3{Y: -0.4375}}
	m.Restore(s)

	const maxTicks = 5 * TickRate
	sawPoint := false
	for i := 0; i < maxTicks && !sawPoint; i++ {
		ev := m.Step([2]PlayerInput{{}, {}})
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
	s.Rally = RallyState{LastKicker: 0, BouncesOnSide: [2]int{0, 1}}
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
	s.Rally = RallyState{LastKicker: 0, BouncesOnSide: [2]int{0, 0}}
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
	inputs := [2]PlayerInput{{MoveX: 0.5, AimX: 1}, {MoveX: -0.5, AimX: -1}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m.Step(inputs)
	}
}

// FuzzStep feeds arbitrary (including NaN/Inf) input into Step and asserts
// the simulation never panics and the resulting positions stay finite.
func FuzzStep(f *testing.F) {
	f.Add(0.0, 0.0, false, 0.0, 0.0, 0.0, 0.0)
	f.Add(math.NaN(), math.Inf(1), true, math.Inf(-1), math.NaN(), 1e300, -1e300)
	f.Add(1.0, 1.0, true, 1.0, 1.0, 1.0, 1.0)

	f.Fuzz(func(t *testing.T, moveX, moveZ float64, kick bool, aimX, aimZ, loft, power float64) {
		m := NewMatch(DefaultConfig())
		in := PlayerInput{MoveX: moveX, MoveZ: moveZ, Kick: kick, AimX: aimX, AimZ: aimZ, Loft: loft, Power: power}

		for i := 0; i < 10; i++ {
			m.Step([2]PlayerInput{in, in})
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
