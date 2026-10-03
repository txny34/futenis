package sim

import "testing"

// --- PredictBall: zero ticks / plain flight (no bounce) --------------------

func TestPredictBall_ZeroTicksIsIdentity2(t *testing.T) {
	cfg := DefaultConfig()
	b := Ball{Pos: Vec3{X: 1, Y: 2, Z: 3}, Vel: Vec3{X: 4, Y: 5, Z: 6}}
	got := PredictBall(b, &cfg, 0)
	if got != b {
		t.Fatalf("PredictBall(0 ticks): got %+v, want %+v unchanged", got, b)
	}
}

// --- PredictBall: now bounce-aware ------------------------------------------

// TestPredictBall_MatchesStepBallFloorBounce is the core bounce-aware
// contract: PredictBall run for N ticks from a ball falling toward the
// floor must land on EXACTLY the same Pos/Vel/Spin a real State's stepBall
// (physics.go) produces over the same N ticks -- same net/floor/wall
// resolution, just without State mutation or events.
func TestPredictBall_MatchesStepBallFloorBounce(t *testing.T) {
	cfg := DefaultConfig()
	start := Ball{Pos: Vec3{Y: 1.0}, Vel: Vec3{X: 2, Y: -0.1}}

	var s State
	s.Cfg = cfg
	s.Ball = start
	const n = 60
	for i := 0; i < n; i++ {
		s.Tick++
		stepBall(&s, nil)
	}

	got := PredictBall(start, &cfg, n)

	if got.Pos != s.Ball.Pos {
		t.Errorf("Pos: got %+v, want %+v", got.Pos, s.Ball.Pos)
	}
	if got.Vel != s.Ball.Vel {
		t.Errorf("Vel: got %+v, want %+v", got.Vel, s.Ball.Vel)
	}
	if got.Spin != s.Ball.Spin {
		t.Errorf("Spin: got %v, want %v", got.Spin, s.Ball.Spin)
	}
}

// TestPredictBall_MatchesStepBallWallBounce covers the wall-reflection path
// specifically (a ball shot straight down a sideline).
func TestPredictBall_MatchesStepBallWallBounce(t *testing.T) {
	cfg := DefaultConfig()
	start := Ball{Pos: Vec3{Y: 0.5}, Vel: Vec3{X: 1, Z: 6}}

	var s State
	s.Cfg = cfg
	s.Ball = start
	const n = 40
	for i := 0; i < n; i++ {
		s.Tick++
		stepBall(&s, nil)
	}

	got := PredictBall(start, &cfg, n)
	if got.Pos != s.Ball.Pos || got.Vel != s.Ball.Vel {
		t.Fatalf("got %+v, want Pos=%+v Vel=%+v", got, s.Ball.Pos, s.Ball.Vel)
	}
}

func TestPredictBall_DoesNotAllocate(t *testing.T) {
	cfg := DefaultConfig()
	b := Ball{Pos: Vec3{Y: 2}, Vel: Vec3{X: 3, Z: -2}}

	allocs := testing.AllocsPerRun(50, func() {
		PredictBall(b, &cfg, 30)
	})
	if allocs != 0 {
		t.Errorf("PredictBall allocated %v times per call, want 0", allocs)
	}
}

// --- PredictLanding ----------------------------------------------------

func TestPredictLanding_DroppedBallLandsAtOrigin(t *testing.T) {
	cfg := DefaultConfig()
	s := State{Cfg: cfg, Ball: Ball{Pos: Vec3{Y: 1.0}}}

	pos, ticks, ok := PredictLanding(&s)
	if !ok {
		t.Fatalf("PredictLanding: got ok=false, want true for a ball dropped straight down")
	}
	if ticks <= 0 {
		t.Errorf("ticks: got %d, want > 0", ticks)
	}
	assertFloat(t, "landing X", pos.X, 0)
	assertFloat(t, "landing Z", pos.Z, 0)
	assertFloat(t, "landing Y", pos.Y, BallRadius)

	// Cross-check against manually stepping PredictBall tick by tick to the
	// same landing tick: must match (not just "close").
	manual := PredictBall(s.Ball, &cfg, ticks)
	if manual.Pos != pos {
		t.Errorf("PredictLanding pos %+v != PredictBall(ticks) pos %+v", pos, manual.Pos)
	}
}

func TestPredictLanding_FlatTrajectoryNeverLandsWithinHorizonIsNotOk(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Gravity = 0.1 // nearly weightless: won't fall BallRadius within the horizon
	cfg.Sanitize()
	s := State{Cfg: cfg, Ball: Ball{Pos: Vec3{Y: 50}}}

	_, _, ok := PredictLanding(&s)
	if ok {
		t.Errorf("PredictLanding: got ok=true, want false (ball never reaches the floor within the horizon)")
	}
}

func TestPredictLanding_DoesNotAllocate(t *testing.T) {
	cfg := DefaultConfig()
	s := State{Cfg: cfg, Ball: Ball{Pos: Vec3{Y: 1.0}}}

	allocs := testing.AllocsPerRun(20, func() {
		PredictLanding(&s)
	})
	if allocs != 0 {
		t.Errorf("PredictLanding allocated %v times per call, want 0", allocs)
	}
}

// --- PredictContact ----------------------------------------------------

func TestPredictContact_BallFallingOntoPlayerFootIsReachable(t *testing.T) {
	cfg := DefaultConfig()
	s := State{Cfg: cfg}
	s.Players[0] = Player{Pos: Vec3{}, OnGround: true}
	// Ball falling nearly straight down onto the player's foot band
	// (PartFoot HIdeal=0.20): starts at the ideal height, zero horizontal
	// velocity, so it should be "in reach" immediately (tick 1) rather than
	// after any real fall.
	s.Ball = Ball{Pos: Vec3{Y: cfg.Parts[PartFoot].HIdeal}}

	ticks, part, quality, ok := PredictContact(&s, 0)
	if !ok {
		t.Fatalf("PredictContact: got ok=false, want true")
	}
	if part != PartFoot {
		t.Errorf("part: got %v, want PartFoot", part)
	}
	if quality != QualPerfect {
		t.Errorf("quality: got %v, want QualPerfect (dead center of the foot band, dH=0)", quality)
	}
	if ticks != 1 {
		t.Errorf("ticks: got %d, want 1 (already in range on the very first predicted tick)", ticks)
	}
}

func TestPredictContact_OutOfReachIsNotOk(t *testing.T) {
	cfg := DefaultConfig()
	s := State{Cfg: cfg}
	s.Players[0] = Player{Pos: Vec3{X: -100}, OnGround: true}
	s.Ball = Ball{Pos: Vec3{X: 100, Y: 5}} // flying away, nowhere near the player

	_, _, _, ok := PredictContact(&s, 0)
	if ok {
		t.Errorf("PredictContact: got ok=true, want false (ball never enters the player's envelope)")
	}
}

func TestPredictContact_InvalidPlayerIndexIsNotOk(t *testing.T) {
	cfg := DefaultConfig()
	s := State{Cfg: cfg}
	for _, idx := range []int{-1, MaxPlayers} {
		_, part, _, ok := PredictContact(&s, idx)
		if ok || part != PartNone {
			t.Errorf("PredictContact(%d): got ok=%v part=%v, want false/PartNone", idx, ok, part)
		}
	}
}

func TestPredictContact_DoesNotAllocate(t *testing.T) {
	cfg := DefaultConfig()
	s := State{Cfg: cfg}
	s.Players[0] = Player{Pos: Vec3{}, OnGround: true}
	s.Ball = Ball{Pos: Vec3{Y: cfg.Parts[PartFoot].HIdeal}}

	allocs := testing.AllocsPerRun(20, func() {
		PredictContact(&s, 0)
	})
	if allocs != 0 {
		t.Errorf("PredictContact allocated %v times per call, want 0", allocs)
	}
}

// --- snapshot wiring -----------------------------------------------------

func TestWriteSnapshot_PredictionBlock(t *testing.T) {
	cfg := DefaultConfig()
	s := State{Cfg: cfg, NumPlayers: 2}
	s.Players[0] = Player{Pos: Vec3{}, OnGround: true}
	s.Players[1] = Player{Pos: Vec3{X: 100}, OnGround: true}
	s.Ball = Ball{Pos: Vec3{Y: cfg.Parts[PartFoot].HIdeal}}

	dst := make([]float64, SnapLen)
	WriteSnapshot(&s, dst)

	if dst[SnapP0ContactOk] != 1 {
		t.Errorf("SnapP0ContactOk: got %v, want 1", dst[SnapP0ContactOk])
	}
	if BodyPart(dst[SnapP0ContactPart]) != PartFoot {
		t.Errorf("SnapP0ContactPart: got %v, want PartFoot", dst[SnapP0ContactPart])
	}
	if dst[SnapP1ContactOk] != 0 {
		t.Errorf("SnapP1ContactOk: got %v, want 0 (player 1 is nowhere near the ball)", dst[SnapP1ContactOk])
	}
	// NumPlayers=2: slots 2/3 are inactive and must stay at their zero value.
	if dst[SnapP2ContactOk] != 0 || dst[SnapP3ContactOk] != 0 {
		t.Errorf("inactive slots' ContactOk: got %v/%v, want 0/0", dst[SnapP2ContactOk], dst[SnapP3ContactOk])
	}
}

func TestWriteSnapshot_PredictionBlockDoesNotAllocate(t *testing.T) {
	cfg := DefaultConfig()
	s := State{Cfg: cfg, NumPlayers: 2, Ball: Ball{Pos: Vec3{Y: 1}}}
	dst := make([]float64, SnapLen)

	allocs := testing.AllocsPerRun(20, func() {
		WriteSnapshot(&s, dst)
	})
	if allocs != 0 {
		t.Errorf("WriteSnapshot allocated %v times per call, want 0", allocs)
	}
}
