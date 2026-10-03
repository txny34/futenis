package sim

import (
	"math"
	"testing"
)

// floatField points at one scalar float64 Config field, paired with the
// inclusive range Sanitize must clamp it into. Used by
// TestConfigSanitize_EveryScalarFieldClampedToRange below so each of the
// ~45 tunable floats gets the same ±1e9 abuse without 45 hand-written
// near-duplicate test functions (code-review hardening, finding #1).
type floatField struct {
	name     string
	ptr      func(c *Config) *float64
	min, max float64
}

// floatFields lists every scalar float64 Config field Sanitize bounds,
// mirroring config_layout.go's scalar order. Parts fields are covered
// separately (TestConfigSanitize_PartsFieldsClampedToRange) since they live
// in an array, not directly on Config.
func floatFields() []floatField {
	return []floatField{
		{"Gravity", func(c *Config) *float64 { return &c.Gravity }, minGravity, maxGravity},
		{"FloorRestitution", func(c *Config) *float64 { return &c.FloorRestitution }, 0, 1},
		{"WallRestitution", func(c *Config) *float64 { return &c.WallRestitution }, 0, 1},
		{"FloorFriction", func(c *Config) *float64 { return &c.FloorFriction }, 0, maxFriction},
		{"AirDrag", func(c *Config) *float64 { return &c.AirDrag }, minAirDrag, maxAirDrag},

		{"PlayerSpeed", func(c *Config) *float64 { return &c.PlayerSpeed }, 0, maxPlayerSpeed},
		{"SprintSpeed", func(c *Config) *float64 { return &c.SprintSpeed }, 0, maxPlayerSpeed},
		{"PlayerAccel", func(c *Config) *float64 { return &c.PlayerAccel }, minPlayerAccel, maxPlayerAccel},
		{"PlayerAirAccel", func(c *Config) *float64 { return &c.PlayerAirAccel }, minPlayerAccel, maxPlayerAccel},
		{"PlayerGravity", func(c *Config) *float64 { return &c.PlayerGravity }, minGravity, maxGravity},
		{"JumpSpeed", func(c *Config) *float64 { return &c.JumpSpeed }, minJumpSpeed, maxJumpSpeed},

		{"StaminaDrain", func(c *Config) *float64 { return &c.StaminaDrain }, minStaminaRate, maxStaminaRate},
		{"StaminaRegen", func(c *Config) *float64 { return &c.StaminaRegen }, minStaminaRate, maxStaminaRate},
		{"StaminaLockTime", func(c *Config) *float64 { return &c.StaminaLockTime }, 0, maxDurationSec},
		{"JumpStaminaCost", func(c *Config) *float64 { return &c.JumpStaminaCost }, 0, 1},

		{"ChargeMaxTime", func(c *Config) *float64 { return &c.ChargeMaxTime }, 0, maxDurationSec},
		{"ChargeMoveFactor", func(c *Config) *float64 { return &c.ChargeMoveFactor }, 0, 1},
		{"StrikeBufferTime", func(c *Config) *float64 { return &c.StrikeBufferTime }, 0, maxBufferSec},

		{"BodyRestitution", func(c *Config) *float64 { return &c.BodyRestitution }, 0, 1},
		{"BodyGraceTime", func(c *Config) *float64 { return &c.BodyGraceTime }, minBodyGraceSec, maxBodyGraceSec},

		{"ServeTossHeight", func(c *Config) *float64 { return &c.ServeTossHeight }, minServeGeom, maxServeGeom},
		{"ServeTossOffset", func(c *Config) *float64 { return &c.ServeTossOffset }, minServeGeom, maxServeGeom},
		{"ServeTossSpeed", func(c *Config) *float64 { return &c.ServeTossSpeed }, 0, maxServeTossSpeed},
		{"ServiceLineDist", func(c *Config) *float64 { return &c.ServiceLineDist }, minServiceLineDist, maxServiceLineDist},
		{"ServeTimeLimit", func(c *Config) *float64 { return &c.ServeTimeLimit }, 0, maxDurationSec},

		{"MomentumFactor", func(c *Config) *float64 { return &c.MomentumFactor }, 0, maxMomentumFactor},
		{"MomentumMax", func(c *Config) *float64 { return &c.MomentumMax }, 0, maxMomentumMax},

		{"DriveNetClearance", func(c *Config) *float64 { return &c.DriveNetClearance }, 0, maxNetClearance},

		{"QualPerfectMax", func(c *Config) *float64 { return &c.QualPerfectMax }, 0, 1},
		{"DevPowerMax", func(c *Config) *float64 { return &c.DevPowerMax }, 0, 1},

		{"SpinFloorFriction", func(c *Config) *float64 { return &c.SpinFloorFriction }, -maxSpinDelta, maxSpinDelta},
		{"SpinFloorRest", func(c *Config) *float64 { return &c.SpinFloorRest }, -maxSpinDelta, maxSpinDelta},
		{"SpinWallRest", func(c *Config) *float64 { return &c.SpinWallRest }, -maxSpinDelta, maxSpinDelta},
		{"SpinWallDrop", func(c *Config) *float64 { return &c.SpinWallDrop }, 0, maxSpinWallDrop},
		{"SpinDecay", func(c *Config) *float64 { return &c.SpinDecay }, 0, 1},

		{"PointPauseTime", func(c *Config) *float64 { return &c.PointPauseTime }, 0, maxDurationSec},

		{"KickReach", func(c *Config) *float64 { return &c.KickReach }, 0, maxKickReach},
	}
}

// TestConfigSanitize_EveryScalarFieldClampedToRange is the code-review
// table test (finding #1): every scalar duration/count/physics field, set
// to -1e9 and then +1e9, must land inside its documented [min,max] after
// Sanitize, and a Step afterwards must stay finite (no NaN/Inf from a
// wrapped tick count or a degenerate physics constant).
func TestConfigSanitize_EveryScalarFieldClampedToRange(t *testing.T) {
	for _, f := range floatFields() {
		for _, abuse := range []float64{-1e9, 1e9} {
			t.Run(f.name, func(t *testing.T) {
				c := DefaultConfig()
				*f.ptr(&c) = abuse
				c.Sanitize()

				got := *f.ptr(&c)
				if got < f.min || got > f.max {
					t.Fatalf("%s=%v after Sanitize: got %v, want in [%v, %v]", f.name, abuse, got, f.min, f.max)
				}
				assertStepStaysFinite(t, c)
			})
		}
	}
}

// TestConfigSanitize_TargetMinFromNetNeverExceedsWallMargin is the
// cross-field guard (finding #1): a pathological TargetWallMargin must
// never leave sanitizeTargetX's own clamp(v, lo, hi) call with lo > hi.
func TestConfigSanitize_TargetMinFromNetNeverExceedsWallMargin(t *testing.T) {
	c := DefaultConfig()
	c.TargetWallMargin = 1e9
	c.TargetMinFromNet = 1e9
	c.Sanitize()

	maxAbs := CourtHalfLength - c.TargetWallMargin
	if c.TargetMinFromNet > maxAbs {
		t.Fatalf("TargetMinFromNet=%v > CourtHalfLength-TargetWallMargin=%v", c.TargetMinFromNet, maxAbs)
	}
	assertStepStaysFinite(t, c)
}

// TestConfigSanitize_LobApexMaxNeverBelowMin and
// TestConfigSanitize_QualGoodMaxNeverBelowPerfectMax guard the two ordered
// pairs among the scalar fields (the rest -- KickMin/MaxSpeed, every Parts
// pair -- are exercised by TestConfigSanitize_PartsFieldsClampedToRange and
// the KickSpeed case below).
func TestConfigSanitize_LobApexMaxNeverBelowMin(t *testing.T) {
	c := DefaultConfig()
	c.LobApexMin = 15
	c.LobApexMax = -1e9
	c.Sanitize()
	if c.LobApexMax < c.LobApexMin {
		t.Fatalf("LobApexMax=%v < LobApexMin=%v", c.LobApexMax, c.LobApexMin)
	}
}

func TestConfigSanitize_QualGoodMaxNeverBelowPerfectMax(t *testing.T) {
	c := DefaultConfig()
	c.QualPerfectMax = 0.9
	c.QualGoodMax = -1e9
	c.Sanitize()
	if c.QualGoodMax < c.QualPerfectMax {
		t.Fatalf("QualGoodMax=%v < QualPerfectMax=%v", c.QualGoodMax, c.QualPerfectMax)
	}
}

func TestConfigSanitize_KickMaxSpeedNeverBelowMinSpeed(t *testing.T) {
	c := DefaultConfig()
	c.KickMinSpeed = 1e9
	c.KickMaxSpeed = -1e9
	c.Sanitize()
	if c.KickMaxSpeed < c.KickMinSpeed {
		t.Fatalf("KickMaxSpeed=%v < KickMinSpeed=%v", c.KickMaxSpeed, c.KickMinSpeed)
	}
	if c.KickMinSpeed > maxKickSpeed {
		t.Fatalf("KickMinSpeed=%v > maxKickSpeed=%v", c.KickMinSpeed, maxKickSpeed)
	}
}

// TestConfigSanitize_PointsToWinClampedToRange covers the one non-float
// scalar (int) field; PointsToWin already had a min-1 regression test
// (TestConfigSanitize_ClampsOutOfRangeFields), this adds the max side and
// the ±1e9 abuse.
func TestConfigSanitize_PointsToWinClampedToRange(t *testing.T) {
	for _, abuse := range []int{-1000000000, 1000000000} {
		c := DefaultConfig()
		c.PointsToWin = abuse
		c.Sanitize()
		if c.PointsToWin < minPointsToWin || c.PointsToWin > maxPointsToWin {
			t.Fatalf("PointsToWin=%v after Sanitize: got %v, want in [%v, %v]", abuse, c.PointsToWin, minPointsToWin, maxPointsToWin)
		}
	}
}

// TestConfigSanitize_ModeOutOfRangeFallsBackToMode1v1 guards the enum field:
// a malformed wire Mode must never reach numPlayersForMode/Parts indexing
// with an out-of-range value.
func TestConfigSanitize_ModeOutOfRangeFallsBackToMode1v1(t *testing.T) {
	c := DefaultConfig()
	c.Mode = Mode(250)
	c.Sanitize()
	if c.Mode != Mode1v1 {
		t.Fatalf("Mode: got %v, want Mode1v1 fallback", c.Mode)
	}
}

// TestConfigSanitize_PartsFieldsClampedToRange is the Parts-table version of
// the main scalar test: every part's height/reach/speed/deviation/spin
// fields, abused at ±1e9, must land in range and keep their ordered-pair
// invariants (HMin<=HIdeal<=HMax, SpeedMin<=SpeedMax, DevBase<=DevMax,
// SpinMin<=SpinMax).
func TestConfigSanitize_PartsFieldsClampedToRange(t *testing.T) {
	for part := PartFoot; part < PartCount; part++ {
		for _, abuse := range []float64{-1e9, 1e9} {
			c := DefaultConfig()
			p := &c.Parts[part]
			p.HMin, p.HIdeal, p.HMax = abuse, abuse, abuse
			p.Reach = abuse
			p.SpeedMin, p.SpeedMax = abuse, abuse
			p.DevBase, p.DevMax = abuse, abuse
			p.SpinMin, p.SpinMax = abuse, abuse

			c.Sanitize()

			got := &c.Parts[part]
			assertInRange(t, "HMin", got.HMin, 0, maxPartHeight)
			assertInRange(t, "HMax", got.HMax, 0, maxPartHeight)
			if got.HIdeal < got.HMin || got.HIdeal > got.HMax {
				t.Errorf("part %d HIdeal=%v out of [HMin=%v, HMax=%v]", part, got.HIdeal, got.HMin, got.HMax)
			}
			assertInRange(t, "Reach", got.Reach, 0, maxPartReach)
			assertInRange(t, "SpeedMin", got.SpeedMin, 0, maxPartSpeed)
			assertInRange(t, "SpeedMax", got.SpeedMax, 0, maxPartSpeed)
			if got.SpeedMax < got.SpeedMin {
				t.Errorf("part %d SpeedMax=%v < SpeedMin=%v", part, got.SpeedMax, got.SpeedMin)
			}
			if got.DevMax < got.DevBase {
				t.Errorf("part %d DevMax=%v < DevBase=%v", part, got.DevMax, got.DevBase)
			}
			if got.SpinMax < got.SpinMin {
				t.Errorf("part %d SpinMax=%v < SpinMin=%v", part, got.SpinMax, got.SpinMin)
			}
			assertInRange(t, "SpinMin", got.SpinMin, -1, 1)
			assertInRange(t, "SpinMax", got.SpinMax, -1, 1)

			assertStepStaysFinite(t, c)
		}
	}
}

// --- cast-site defense-in-depth (finding #1) -----------------------------

// TestClampTickCount covers clampTickCount's own contract directly, since
// it is the last line of defense at every *Ticks cast site (movement.go,
// body.go) against a negative int wrapping into a huge unsigned value.
func TestClampTickCount(t *testing.T) {
	cases := []struct {
		n, max, want int
	}{
		{-1, 100, 0},
		{-1000000000, math.MaxUint16, 0},
		{50, 100, 50},
		{1000000000, math.MaxUint16, math.MaxUint16},
	}
	for _, c := range cases {
		if got := clampTickCount(c.n, c.max); got != c.want {
			t.Errorf("clampTickCount(%d, %d): got %d, want %d", c.n, c.max, got, c.want)
		}
	}
}

// --- helpers --------------------------------------------------------------

// assertInRange fails with a descriptive message if got is outside [min,max].
func assertInRange(t *testing.T, name string, got, min, max float64) {
	t.Helper()
	if got < min || got > max {
		t.Errorf("%s: got %v, want in [%v, %v]", name, got, min, max)
	}
}

// assertStepStaysFinite drives a fresh Match with the given (already
// Sanitized) Config for a couple hundred ticks of idle input and fails if
// the ball or any active player's position/velocity ever goes NaN/Inf --
// the actual symptom a wrapped tick count or a degenerate physics constant
// would eventually produce.
func assertStepStaysFinite(t *testing.T, cfg Config) {
	t.Helper()
	m := NewMatch(cfg)
	var in [MaxPlayers]PlayerInput
	for i := 0; i < 200; i++ {
		m.Step(in)
		s := m.State()
		if !finiteVec(s.Ball.Pos) || !finiteVec(s.Ball.Vel) {
			t.Fatalf("tick %d: ball went non-finite: pos=%+v vel=%+v", i, s.Ball.Pos, s.Ball.Vel)
		}
		for p := int8(0); p < s.NumPlayers; p++ {
			pl := s.Players[p]
			if !finiteVec(pl.Pos) || !finiteVec(pl.Vel) {
				t.Fatalf("tick %d: player %d went non-finite: pos=%+v vel=%+v", i, p, pl.Pos, pl.Vel)
			}
		}
	}
}

// finiteVec reports whether every component of v is a finite float64.
func finiteVec(v Vec3) bool {
	return !math.IsNaN(v.X) && !math.IsInf(v.X, 0) &&
		!math.IsNaN(v.Y) && !math.IsInf(v.Y, 0) &&
		!math.IsNaN(v.Z) && !math.IsInf(v.Z, 0)
}
