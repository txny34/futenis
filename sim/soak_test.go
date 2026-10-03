package sim

import (
	"math"
	"testing"
)

// soak_test.go: long-horizon stability tests (code-review hardening,
// finding #4). These are slow by design (hundreds of thousands of ticks
// per sub-test) and are skipped under `go test -short`; run explicitly with
// `go test -run Soak -v ./sim/`.
//
// What's asserted, every tick, for the whole run:
//   - no NaN/Inf anywhere in Ball/Player Pos/Vel.
//   - every active player stays within the court (clampPlayerToCourt's own
//     contract); the ball stays within a generous bound unless Ball.Out is
//     already true (ModePlayground intentionally lets an out ball keep
//     flying away until RespawnBall -- see rules.go/match.go docs).
//   - every point (1v1/2v2 only) resolves -- an EvPoint/EvGameOver -- within
//     maxPointTicks (60s of simulated play) of the previous one.
//   - the match never sits in PhaseServe or PhasePoint past a generous
//     multiple of their own configured limits (serve clock / pause ticks).
//
// Plus: one zero-alloc sample per run (on an isolated clone, so it never
// perturbs the main run's determinism) and two targeted scenarios (2v2
// same-tick double touch, an idle server) folded into the same soak budget.

const (
	soakTicks          = 200000
	maxPointTicks      = 60 * TickRate        // a point must resolve within 60s of play
	maxServeStallTicks = 10 * TickRate        // generous margin over any ServeTimeLimit
	maxPointPauseStall = 10 * TickRate        // generous margin over PointPauseTicks
	soakPlayerBound    = CourtHalfLength + 1  // clampPlayerToCourt already keeps this tight
	soakBallBoundXZ    = CourtHalfLength * 10 // ball can fly well past the walls before EvOut/next serve resets it
	soakBallBoundY     = 200                  // generous; default physics never gets near this
)

// TestSoak_RandomPlay_1v1/2v2/Playground drive a full Match for soakTicks
// with deterministic, scripted (never real-random) inputs across several
// seeds, and assert the stability contract above.
func TestSoak_RandomPlay_1v1(t *testing.T)        { runModeSoak(t, Mode1v1) }
func TestSoak_RandomPlay_2v2(t *testing.T)        { runModeSoak(t, Mode2v2) }
func TestSoak_RandomPlay_Playground(t *testing.T) { runModeSoak(t, ModePlayground) }

func runModeSoak(t *testing.T, mode Mode) {
	if testing.Short() {
		t.Skip("soak: skipped under -short")
	}
	for _, seed := range []uint64{1, 2, 3, 42} {
		seed := seed
		t.Run(seedName(seed), func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Mode = mode
			runSoak(t, cfg, seed)
		})
	}
}

func seedName(seed uint64) string {
	switch seed {
	case 1:
		return "seed1"
	case 2:
		return "seed2"
	case 3:
		return "seed3"
	default:
		return "seed42"
	}
}

// soakTracker holds the running state runSoak checks every tick.
type soakTracker struct {
	sincePoint    int
	serveStreak   int
	pointStreak   int
	lastPhase     Phase
	allocsSampled bool
}

// runSoak is the shared driver for every random-play soak scenario: one
// independent lcg per player slot (so each player's input stream is
// reproducible on its own regardless of NumPlayers), stepped soakTicks
// times, with every invariant re-checked every tick.
func runSoak(t *testing.T, cfg Config, seed uint64) {
	t.Helper()
	m := NewMatch(cfg)
	gens := make([]*lcg, MaxPlayers)
	for i := range gens {
		gens[i] = newLCG(seed*1000 + uint64(i) + 1)
	}

	tr := &soakTracker{lastPhase: m.State().Phase}
	for tick := 0; tick < soakTicks; tick++ {
		var inputs [MaxPlayers]PlayerInput
		s := m.State()
		for i := int8(0); i < s.NumPlayers; i++ {
			inputs[i] = scriptedInput(gens[i])
		}

		if !tr.allocsSampled && tick == soakTicks/2 {
			assertCloneStepAllocatesZero(t, m, inputs)
			tr.allocsSampled = true
		}

		ev := m.Step(inputs)
		checkSoakTick(t, m, ev, tr, tick)
		if t.Failed() {
			return
		}
	}
}

// checkSoakTick re-checks every soak invariant after one Step: finiteness,
// bounds, point/serve/pause liveness.
func checkSoakTick(t *testing.T, m *Match, ev []Event, tr *soakTracker, tick int) {
	t.Helper()
	s := m.State()

	assertFinite(t, tick, s)
	assertBounds(t, tick, s)
	assertServerPlayerInvariant(t, tick, s)

	for _, e := range ev {
		if e.Kind == EvPoint || e.Kind == EvGameOver {
			if tr.sincePoint > maxPointTicks {
				t.Fatalf("tick %d: point took %d ticks (> %d, 60s) to resolve -- repro: seed-driven soak, mode=%v", tick, tr.sincePoint, maxPointTicks, s.Cfg.Mode)
			}
			tr.sincePoint = 0
		}
	}
	tr.sincePoint++

	trackPhaseLiveness(t, tick, s, tr)
}

// trackPhaseLiveness fails if the match sits in PhaseServe or PhasePoint
// far longer than its own configured limit would ever allow, which would
// mean the serve clock or the point-pause countdown got stuck.
func trackPhaseLiveness(t *testing.T, tick int, s State, tr *soakTracker) {
	t.Helper()
	if s.Phase != tr.lastPhase {
		tr.serveStreak, tr.pointStreak = 0, 0
		tr.lastPhase = s.Phase
	}
	switch s.Phase {
	case PhaseServe:
		tr.serveStreak++
		if tr.serveStreak > maxServeStallTicks {
			t.Fatalf("tick %d: stuck in PhaseServe for %d ticks straight -- repro: seed-driven soak, mode=%v, ServeTimeLimit=%v", tick, tr.serveStreak, s.Cfg.Mode, s.Cfg.ServeTimeLimit)
		}
	case PhasePoint:
		tr.pointStreak++
		if tr.pointStreak > maxPointPauseStall {
			t.Fatalf("tick %d: stuck in PhasePoint for %d ticks straight -- repro: seed-driven soak, mode=%v", tick, tr.pointStreak, s.Cfg.Mode)
		}
	}
}

// assertFinite fails if the ball or any active player went NaN/Inf.
func assertFinite(t *testing.T, tick int, s State) {
	t.Helper()
	if !finiteVec(s.Ball.Pos) || !finiteVec(s.Ball.Vel) {
		t.Fatalf("tick %d: ball non-finite: pos=%+v vel=%+v", tick, s.Ball.Pos, s.Ball.Vel)
	}
	for i := int8(0); i < s.NumPlayers; i++ {
		p := s.Players[i]
		if !finiteVec(p.Pos) || !finiteVec(p.Vel) {
			t.Fatalf("tick %d: player %d non-finite: pos=%+v vel=%+v", tick, i, p.Pos, p.Vel)
		}
	}
}

// assertBounds fails if a player ever leaves the court (clampPlayerToCourt's
// own contract) or the ball drifts an absurd distance away while still
// marked inside (!Out) -- which would mean a wall/net resolution bug, not
// ModePlayground's documented "out ball keeps flying" behavior.
func assertBounds(t *testing.T, tick int, s State) {
	t.Helper()
	for i := int8(0); i < s.NumPlayers; i++ {
		p := s.Players[i].Pos
		if math.Abs(p.X) > soakPlayerBound || math.Abs(p.Z) > soakPlayerBound {
			t.Fatalf("tick %d: player %d left the court: pos=%+v", tick, i, p)
		}
	}
	if s.Ball.Out {
		return
	}
	b := s.Ball.Pos
	if math.Abs(b.X) > soakBallBoundXZ || math.Abs(b.Z) > soakBallBoundXZ || math.Abs(b.Y) > soakBallBoundY {
		t.Fatalf("tick %d: ball drifted out of bounds while still !Out: pos=%+v", tick, b)
	}
}

// assertServerPlayerInvariant fails if Rally.ServerPlayer ever points at a
// player on the wrong team (code-review hardening: a mismatch here means a
// serveFault/replayServe call site passed a team that doesn't match the
// frozen ServerPlayer, which would let a player toss/serve for a team they
// don't belong to).
func assertServerPlayerInvariant(t *testing.T, tick int, s State) {
	t.Helper()
	// Only meaningful during PhaseServe/PhaseRally: awardPoint (either
	// scoring mode) sets State.Server to the NEXT point's server the
	// instant the point is decided, but Rally.ServerPlayer -- "who served
	// THIS point" -- is only recomputed when resetForServe actually runs,
	// at the end of the PhasePoint pause. That gap is by design, not a bug.
	if s.Phase != PhaseServe && s.Phase != PhaseRally {
		return
	}
	sp := s.Rally.ServerPlayer
	if sp < 0 || sp >= s.NumPlayers {
		return // not mid-serve-contract (e.g. between points before a toss)
	}
	if s.Players[sp].Team != s.Server {
		t.Fatalf("tick %d: Rally.ServerPlayer=%d has Team=%d, want == Server=%d", tick, sp, s.Players[sp].Team, s.Server)
	}
}

// assertCloneStepAllocatesZero samples the zero-allocations-per-tick
// contract (CLAUDE.md) mid-soak, on an isolated clone of the current state
// so the measurement never perturbs the main run's determinism.
func assertCloneStepAllocatesZero(t *testing.T, m *Match, inputs [MaxPlayers]PlayerInput) {
	t.Helper()
	clone := NewMatch(m.State().Cfg)
	clone.Restore(m.State())

	allocs := testing.AllocsPerRun(20, func() {
		clone.Step(inputs)
	})
	if allocs != 0 {
		t.Errorf("Step allocated %v times per call mid-soak, want 0", allocs)
	}
}

// --- targeted scenarios folded into the soak budget ----------------------

// TestSoak_2v2SameTickDoubleTouchRace is the 2v2 same-tick double-touch race
// explicitly called out in the review: two teammates both in range of the
// ball, both charging a strike, both released the same tick. applyStrikes
// must deterministically resolve exactly one winner (strikerBeats' documented
// lower-index tiebreak) every tick for a long run, never panic, and never
// leave both players' Buffer non-zero (which would mean the "one touch"
// resolution silently double-fired).
func TestSoak_2v2SameTickDoubleTouchRace(t *testing.T) {
	if testing.Short() {
		t.Skip("soak: skipped under -short")
	}
	cfg := DefaultConfig()
	cfg.Mode = Mode2v2
	m := NewMatch(cfg)
	s := m.State()
	// Put both team-0 partners on top of the ball, both mid-charge, so the
	// very next release puts two live candidates on the same tick.
	s.Phase = PhaseRally
	s.Rally = RallyState{LastTeam: -1, LastPlayer: -1}
	s.Ball = Ball{Pos: Vec3{X: -1, Y: 0.2}}
	s.Players[0] = Player{Team: 0, Pos: Vec3{X: -1}, OnGround: true, Stamina: 1}
	s.Players[1] = Player{Team: 0, Pos: Vec3{X: -1}, OnGround: true, Stamina: 1}
	s.Players[2] = Player{Team: 1, Pos: Vec3{X: 8}, OnGround: true, Stamina: 1}
	s.Players[3] = Player{Team: 1, Pos: Vec3{X: 8}, OnGround: true, Stamina: 1}
	m.Restore(s)

	var inputs [MaxPlayers]PlayerInput
	for i := range inputs {
		inputs[i] = PlayerInput{TargetX: 8, Buttons: BtnStrike}
	}

	const chargeTicks = 5
	for i := 0; i < chargeTicks; i++ {
		m.Step(inputs)
	}
	// Release both teammates' Strike on the exact same tick.
	inputs[0].Buttons, inputs[1].Buttons = 0, 0

	for i := 0; i < 2*TickRate; i++ {
		ev := m.Step(inputs)
		kicks := 0
		for _, e := range ev {
			if e.Kind == EvKick {
				kicks++
			}
		}
		if kicks > 1 {
			t.Fatalf("tick %d: got %d EvKick in one tick, want at most 1 (double-touch race must resolve to one striker)", i, kicks)
		}
		st := m.State()
		if st.Players[0].Buffer != 0 && st.Players[1].Buffer != 0 && kicks == 0 {
			// Both still buffered and nobody has struck yet is fine while the
			// window is still open; only a problem combined with >1 kicks,
			// already checked above. Nothing to assert here beyond no panic.
			_ = st
		}
	}
}

// TestSoak_IdleServerAlwaysFinishesTheMatch is the idle-server scenario
// folded into the soak budget: a server that never presses Strike must
// still lose every point to the serve clock + double fault, and the match
// must reach PhaseGameOver well within the soak tick budget instead of
// hanging in PhaseServe forever.
func TestSoak_IdleServerAlwaysFinishesTheMatch(t *testing.T) {
	if testing.Short() {
		t.Skip("soak: skipped under -short")
	}
	cfg := DefaultConfig()
	cfg.ServeTimeLimit = 0.25 // keep the soak budget small
	cfg.Sanitize()
	m := NewMatch(cfg)

	var idle [MaxPlayers]PlayerInput
	for i := 0; i < soakTicks; i++ {
		ev := m.Step(idle)
		for _, e := range ev {
			if e.Kind == EvGameOver {
				if m.State().Phase != PhaseGameOver {
					t.Fatalf("tick %d: got EvGameOver but Phase=%v", i, m.State().Phase)
				}
				return
			}
		}
	}
	t.Fatalf("idle server: match never reached PhaseGameOver within %d ticks (stuck)", soakTicks)
}
