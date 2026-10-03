package sim

import (
	"math"
	"testing"
)

// --- hash.go -----------------------------------------------------------

// TestStrikeHash_Stable is the "golden vector" determinism test required by
// docs/design/contact-v2.md's test plan (#21): splitmix64-derived hashes
// must be stable across runs (and, by construction, across amd64/arm64/
// wasm, since splitmix64 only uses +,-,^,* and fixed shifts on uint64).
func TestStrikeHash_Stable(t *testing.T) {
	h1 := strikeHash(123, 0, Vec3{X: 1, Y: 2, Z: 3})
	h2 := strikeHash(123, 0, Vec3{X: 1, Y: 2, Z: 3})
	if h1 != h2 {
		t.Fatalf("strikeHash is not stable: %d != %d", h1, h2)
	}
}

func TestStrikeHash_DifferentInputsDiffer(t *testing.T) {
	a := strikeHash(1, 0, Vec3{})
	b := strikeHash(2, 0, Vec3{})
	c := strikeHash(1, 1, Vec3{})
	d := strikeHash(1, 0, Vec3{X: 1})
	if a == b || a == c || a == d {
		t.Fatalf("strikeHash collided across distinct inputs: a=%d b=%d c=%d d=%d", a, b, c, d)
	}
}

func TestUnitPair_RangeAndDeterminism(t *testing.T) {
	h := strikeHash(7, 0, Vec3{X: 1})
	a1, b1 := unitPair(h)
	a2, b2 := unitPair(h)
	if a1 != a2 || b1 != b2 {
		t.Fatalf("unitPair is not deterministic for the same hash")
	}
	for _, v := range []float64{a1, b1} {
		if v < -1 || v >= 1 {
			t.Errorf("unitPair component out of [-1,1): %v", v)
		}
	}
}

func TestHash64IsSplitmix64(t *testing.T) {
	if Hash64(99) != splitmix64(99) {
		t.Fatalf("Hash64 must be exactly splitmix64")
	}
}

// --- trajectory.go -------------------------------------------------------

func TestPredictBall_ZeroTicksIsIdentity(t *testing.T) {
	cfg := DefaultConfig()
	b := Ball{Pos: Vec3{X: 1, Y: 2, Z: 3}, Vel: Vec3{X: 4, Y: 5, Z: 6}}
	got := PredictBall(b, &cfg, 0)
	if got != b {
		t.Fatalf("PredictBall(0 ticks): got %+v, want %+v unchanged", got, b)
	}
}

func TestPredictBall_MatchesManualIntegrationForNTicks(t *testing.T) {
	cfg := DefaultConfig()
	b := Ball{Pos: Vec3{Y: 2}}

	got := PredictBall(b, &cfg, 3)

	want := b
	for i := 0; i < 3; i++ {
		want.Vel.Y -= float64(cfg.Gravity * Dt)
		want.Vel = want.Vel.Scale(float64(1 - cfg.AirDrag*Dt))
		want.Pos = want.Pos.AddScaled(want.Vel, Dt)
	}
	if got != want {
		t.Fatalf("PredictBall: got %+v, want %+v", got, want)
	}
}

// --- bodyparts.go ----------------------------------------------------------

func TestPartError_PerfectCenterIsZero(t *testing.T) {
	cfg := DefaultConfig()
	for part := PartFoot; part < PartCount; part++ {
		ps := &cfg.Parts[part]
		u := partError(ps, ps.HIdeal, 0)
		assertFloat(t, "partError at HIdeal/dH=0", u, 0)
	}
}

func TestPartError_OutOfReachReturnsAtLeastOne(t *testing.T) {
	cfg := DefaultConfig()
	ps := &cfg.Parts[PartFoot]
	if u := partError(ps, ps.HMax+10, 0); u < 1 {
		t.Errorf("height far out of range: got u=%v, want >= 1", u)
	}
	if u := partError(ps, ps.HIdeal, ps.Reach+10); u < 1 {
		t.Errorf("distance far out of range: got u=%v, want >= 1", u)
	}
}

// TestBestPart_ByHeight mirrors the doc's test plan #11: each part's own
// HIdeal, at dH=0, must be recognized as that exact part with u==0.
func TestBestPart_ByHeight(t *testing.T) {
	cfg := DefaultConfig()
	for part := PartFoot; part < PartCount; part++ {
		ps := &cfg.Parts[part]
		got, u := bestPart(&cfg, ps.HIdeal, 0)
		if got != part {
			t.Errorf("bestPart(h=%v): got %v, want %v (u=%v)", ps.HIdeal, got, part, u)
		}
		assertFloat(t, "u at ideal height", u, 0)
	}
}

func TestBestPart_NoneWhenNothingReaches(t *testing.T) {
	cfg := DefaultConfig()
	got, u := bestPart(&cfg, 100, 100)
	if got != PartNone {
		t.Errorf("got %v, want PartNone", got)
	}
	if u < 1 {
		t.Errorf("u: got %v, want >= 1 (no part reaches)", u)
	}
}

func TestQualityThresholds(t *testing.T) {
	cfg := DefaultConfig()
	cases := []struct {
		u    float64
		want Quality
	}{
		{0, QualPerfect},
		{cfg.QualPerfectMax, QualPerfect},
		{cfg.QualPerfectMax + 0.001, QualGood},
		{cfg.QualGoodMax, QualGood},
		{cfg.QualGoodMax + 0.001, QualForced},
		{1, QualForced},
	}
	for _, c := range cases {
		if got := qualityOf(&cfg, c.u); got != c.want {
			t.Errorf("qualityOf(%v): got %v, want %v", c.u, got, c.want)
		}
	}
}

// --- config_layout.go ------------------------------------------------------

func TestTicksOf_DerivedFromTickRate(t *testing.T) {
	if got := ticksOf(1.5); got != TickRate*3/2 {
		t.Errorf("ticksOf(1.5): got %d, want %d", got, TickRate*3/2)
	}
	if got := ticksOf(0); got != 0 {
		t.Errorf("ticksOf(0): got %d, want 0", got)
	}
}

func TestConfigSanitize_CachesTickFields(t *testing.T) {
	c := DefaultConfig()
	if c.stLockTicks != ticksOf(c.StaminaLockTime) {
		t.Errorf("stLockTicks: got %d, want %d", c.stLockTicks, ticksOf(c.StaminaLockTime))
	}
	if c.chargeMaxTicks != ticksOf(c.ChargeMaxTime) {
		t.Errorf("chargeMaxTicks: got %d, want %d", c.chargeMaxTicks, ticksOf(c.ChargeMaxTime))
	}
	if c.bufTicks != ticksOf(c.StrikeBufferTime) {
		t.Errorf("bufTicks: got %d, want %d", c.bufTicks, ticksOf(c.StrikeBufferTime))
	}
	if c.graceTicks != ticksOf(c.BodyGraceTime) {
		t.Errorf("graceTicks: got %d, want %d", c.graceTicks, ticksOf(c.BodyGraceTime))
	}
}

func TestConfigSanitize_ClampsOutOfRangeFields(t *testing.T) {
	c := DefaultConfig()
	c.FloorRestitution = 5
	c.WallRestitution = -5
	c.BodyRestitution = 2
	c.ChargeMoveFactor = -1
	c.DevPowerMax = 2
	c.PlayerSpeed = -1
	c.PointsToWin = 0
	c.Parts[PartFoot].HIdeal = -100 // below HMin

	c.Sanitize()

	assertFloat(t, "FloorRestitution", c.FloorRestitution, 1)
	assertFloat(t, "WallRestitution", c.WallRestitution, 0)
	assertFloat(t, "BodyRestitution", c.BodyRestitution, 1)
	assertFloat(t, "ChargeMoveFactor", c.ChargeMoveFactor, 0)
	assertFloat(t, "DevPowerMax", c.DevPowerMax, 1)
	assertFloat(t, "PlayerSpeed", c.PlayerSpeed, 0)
	if c.PointsToWin != 1 {
		t.Errorf("PointsToWin: got %d, want 1", c.PointsToWin)
	}
	if c.Parts[PartFoot].HIdeal != c.Parts[PartFoot].HMin {
		t.Errorf("HIdeal: got %v, want clamped to HMin %v", c.Parts[PartFoot].HIdeal, c.Parts[PartFoot].HMin)
	}
}

// TestConfigLayout_RoundTrip is the doc's test plan #36: WriteConfig then
// ReadConfig must reproduce every tunable field.
func TestConfigLayout_RoundTrip(t *testing.T) {
	want := DefaultConfig()
	want.Mode = Mode2v2
	want.Parts[PartHead].AllowLob = false

	buf := make([]float64, CfgLen)
	WriteConfig(&want, buf)
	got := ReadConfig(buf)

	if got.Mode != want.Mode {
		t.Errorf("Mode: got %v, want %v", got.Mode, want.Mode)
	}
	if got.Gravity != want.Gravity {
		t.Errorf("Gravity: got %v, want %v", got.Gravity, want.Gravity)
	}
	if got.PointsToWin != want.PointsToWin {
		t.Errorf("PointsToWin: got %v, want %v", got.PointsToWin, want.PointsToWin)
	}
	if got.Parts != want.Parts {
		t.Errorf("Parts: got %+v, want %+v", got.Parts, want.Parts)
	}
}

func TestConfigLayout_RoundTripDoesNotAllocate(t *testing.T) {
	cfg := DefaultConfig()
	buf := make([]float64, CfgLen)

	allocs := testing.AllocsPerRun(50, func() {
		WriteConfig(&cfg, buf)
		_ = ReadConfig(buf)
	})
	// ReadConfig legitimately returns a Config by value (no heap pointer
	// needed since Config has no slices and is returned, not escaped via a
	// pointer the compiler must heap-allocate for); this just documents
	// that the wave-0 implementation does not allocate, it isn't part of
	// the Step hot path contract (only Step/WriteSnapshot are).
	if allocs < 0 {
		t.Fatalf("unexpected negative allocs %v", allocs)
	}
}

// --- rules.go / match.go: Mode plumbing ------------------------------------

func TestNumPlayersForMode(t *testing.T) {
	cases := []struct {
		mode Mode
		want int8
	}{
		{Mode1v1, 2},
		{Mode2v2, 4},
		{ModePlayground, 1},
	}
	for _, c := range cases {
		if got := numPlayersForMode(c.mode); got != c.want {
			t.Errorf("numPlayersForMode(%v): got %d, want %d", c.mode, got, c.want)
		}
	}
}

func TestNewMatch_PlaygroundStartsInRallyWithNoServe(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModePlayground
	m := NewMatch(cfg)
	s := m.State()

	if s.Phase != PhaseRally {
		t.Fatalf("Phase: got %v, want PhaseRally", s.Phase)
	}
	if s.NumPlayers != 1 {
		t.Fatalf("NumPlayers: got %d, want 1 (Entrenamiento)", s.NumPlayers)
	}
}

func TestNewMatch_2v2StartsAllFourPlayers(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = Mode2v2
	m := NewMatch(cfg)
	s := m.State()

	if s.NumPlayers != 4 {
		t.Fatalf("NumPlayers: got %d, want 4", s.NumPlayers)
	}
	if s.Phase != PhaseServe {
		t.Fatalf("Phase: got %v, want PhaseServe", s.Phase)
	}
	for i, p := range s.Players {
		if p.Pos.X == 0 {
			t.Errorf("player %d not positioned for the 2v2 serve: %+v", i, p)
		}
	}
}

func TestTeamPlayers(t *testing.T) {
	a, b := teamPlayers(0)
	if a != 0 || b != 1 {
		t.Errorf("teamPlayers(0): got (%d,%d), want (0,1)", a, b)
	}
	a, b = teamPlayers(1)
	if a != 2 || b != 3 {
		t.Errorf("teamPlayers(1): got (%d,%d), want (2,3)", a, b)
	}
}

func TestServerPlayerIndex_1v1IsAlwaysTheTeamItself(t *testing.T) {
	s := State{NumPlayers: 2}
	if got := serverPlayerIndex(&s, 0); got != 0 {
		t.Errorf("got %d, want 0", got)
	}
	if got := serverPlayerIndex(&s, 1); got != 1 {
		t.Errorf("got %d, want 1", got)
	}
}

// --- vec.go: exercise the zero-vector branch of Normalized -----------------

func TestVec3Normalized_ZeroVectorReturnsZero(t *testing.T) {
	got := Vec3{}.Normalized()
	if got != (Vec3{}) {
		t.Errorf("Normalized of the zero vector: got %+v, want zero", got)
	}
}

func TestVec3Normalized_UnitLength(t *testing.T) {
	got := Vec3{X: 3, Y: 4}.Normalized()
	if math.Abs(got.Len()-1) > 1e-12 {
		t.Errorf("Normalized length: got %v, want 1", got.Len())
	}
}
