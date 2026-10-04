package sim

import "math"

// Sanitize bounds (code-review hardening, docs/design/contact-v2.md §13):
// every tunable field gets a documented min AND max so a malformed
// live-tuning payload (ReadConfig, called from the dev panel / wasm bridge)
// can never produce a negative or absurd value that later wraps when cast
// to an unsigned tick counter (StaminaLock/ChargeTicks are uint16, Buffer is
// uint8, StruckAt/Tick are uint32) or destabilizes the physics/solver.
//
// Duration (`*Time`) and count fields that feed ticksOf are capped well
// inside the destination type's range at TickRate=120: maxDurationSec*120
// never gets close to uint16's 65535, let alone uint32's range, and
// maxBufferSec is chosen to fit uint8 (255 ticks = 2.125 s) directly, since
// freezeBuffer casts bufTicks straight to uint8.
const (
	minGravity, maxGravity                 = 0.1, 50 // m/s^2; never <=0 (solver/lob divide by it)
	maxRestDelta                           = 1       // 0..1, every *Restitution/*Factor already clamped there inline
	maxFriction                            = 1       // fraction of speed kept per bounce
	minAirDrag, maxAirDrag                 = 0, 5
	minPointsToWin, maxPointsToWin         = 1, 99
	maxPlayerSpeed                         = 30 // m/s
	minPlayerAccel, maxPlayerAccel         = 0, 200
	minJumpSpeed, maxJumpSpeed             = 0, 20
	minStaminaRate, maxStaminaRate         = 0, 10 // 1/s
	maxDurationSec                         = 10    // s, generic cap for most *Time fields (see maxBufferSec)
	maxBufferSec                           = float64(math.MaxUint8) / TickRate
	minBodyGraceSec, maxBodyGraceSec       = 0, 5
	minServeGeom, maxServeGeom             = 0, 5                       // m, toss height/offset/box margins
	maxServeTossSpeed                      = 15                         // m/s
	minServiceLineDist, maxServiceLineDist = 0.5, CourtHalfLength - 0.5 // m
	maxMomentumFactor, maxMomentumMax      = 2, 20
	minLobApex, maxLobApex                 = 0.1, 20 // m
	maxNetClearance                        = 5       // m
	maxTargetMinFromNet                    = CourtHalfLength
	maxTargetWallMargin                    = CourtHalfWidth
	maxSpinDelta                           = 2 // generic +/- bound on the spin-bounce deltas
	maxSpinWallDrop                        = 20
	maxKickReach                           = 5
	maxKickSpeed                           = 50
	maxPartHeight                          = 3 // m, relative ball-height band a body part can reach
	maxPartReach                           = 5
	maxPartSpeed                           = 60
	maxPartDev                             = 10
	maxDashSpeed                           = 40 // m/s
	minGamesPerSet, maxGamesPerSet         = 1, 20
	minSetsToWin, maxSetsToWin             = 1, 10
)

// Sanitize clamps Config to physically sane bounds and recomputes the
// cached derived tick counts (docs/design/contact-v2.md §1.1, §13). Called
// by DefaultConfig and ReadConfig; must also be called after any live edit
// from the dev panel, between frames (never mid-Step).
func (c *Config) Sanitize() {
	c.sanitizeBallAndPlayerPhysics()
	c.sanitizeStaminaAndCharge()
	c.sanitizeBodyAndServe()
	c.sanitizeShotShaping()
	c.sanitizeSpinAndMisc()
	c.sanitizeParts()
	c.sanitizeFeed()
	c.recomputeTickCaches()
}

// sanitizeBallAndPlayerPhysics bounds §1's ball physics and §4/§5's player
// movement fields.
func (c *Config) sanitizeBallAndPlayerPhysics() {
	c.Gravity = clamp(c.Gravity, minGravity, maxGravity)
	c.FloorRestitution = clamp(c.FloorRestitution, 0, maxRestDelta)
	c.WallRestitution = clamp(c.WallRestitution, 0, maxRestDelta)
	c.FloorFriction = clamp(c.FloorFriction, 0, maxFriction)
	c.AirDrag = clamp(c.AirDrag, minAirDrag, maxAirDrag)
	if c.PointsToWin < minPointsToWin {
		c.PointsToWin = minPointsToWin
	} else if c.PointsToWin > maxPointsToWin {
		c.PointsToWin = maxPointsToWin
	}

	c.PlayerSpeed = clamp(c.PlayerSpeed, 0, maxPlayerSpeed)
	c.SprintSpeed = clamp(c.SprintSpeed, 0, maxPlayerSpeed)
	c.PlayerAccel = clamp(c.PlayerAccel, minPlayerAccel, maxPlayerAccel)
	c.PlayerAirAccel = clamp(c.PlayerAirAccel, minPlayerAccel, maxPlayerAccel)
	c.PlayerGravity = clamp(c.PlayerGravity, minGravity, maxGravity)
	c.JumpSpeed = clamp(c.JumpSpeed, minJumpSpeed, maxJumpSpeed)
}

// sanitizeStaminaAndCharge bounds §5's stamina drain/regen/lock and §7.1's
// charge/buffer durations.
func (c *Config) sanitizeStaminaAndCharge() {
	c.StaminaDrain = clamp(c.StaminaDrain, minStaminaRate, maxStaminaRate)
	c.StaminaRegen = clamp(c.StaminaRegen, minStaminaRate, maxStaminaRate)
	c.StaminaLockTime = clamp(c.StaminaLockTime, 0, maxDurationSec)
	c.JumpStaminaCost = clamp(c.JumpStaminaCost, 0, 1)

	c.ChargeMaxTime = clamp(c.ChargeMaxTime, 0, maxDurationSec)
	c.ChargeMoveFactor = clamp(c.ChargeMoveFactor, 0, maxRestDelta)
	// StrikeBufferTime is cast straight to a uint8 tick count (freezeBuffer),
	// so its sane range is the type's own range, not the generic duration cap.
	c.StrikeBufferTime = clamp(c.StrikeBufferTime, 0, maxBufferSec)
}

// sanitizeBodyAndServe bounds §7.4's body-collision grace window and §7.5's
// serve geometry/clock.
func (c *Config) sanitizeBodyAndServe() {
	c.BodyRestitution = clamp(c.BodyRestitution, 0, maxRestDelta)
	c.BodyGraceTime = clamp(c.BodyGraceTime, minBodyGraceSec, maxBodyGraceSec)

	c.ServeTossHeight = clamp(c.ServeTossHeight, minServeGeom, maxServeGeom)
	c.ServeTossOffset = clamp(c.ServeTossOffset, minServeGeom, maxServeGeom)
	c.ServeTossSpeed = clamp(c.ServeTossSpeed, 0, maxServeTossSpeed)
	c.ServiceLineDist = clamp(c.ServiceLineDist, minServiceLineDist, maxServiceLineDist)
	c.ServeTimeLimit = clamp(c.ServeTimeLimit, 0, maxDurationSec)
}

// sanitizeShotShaping bounds §8/§9's momentum, lob/drive shaping and target
// clamps -- including the TargetMinFromNet/TargetWallMargin cross-check that
// keeps sanitizeTargetX's own clamp(lo, hi) call in input.go from ever
// seeing lo > hi.
func (c *Config) sanitizeShotShaping() {
	c.MomentumFactor = clamp(c.MomentumFactor, 0, maxMomentumFactor)
	c.MomentumMax = clamp(c.MomentumMax, 0, maxMomentumMax)

	c.LobApexMin = clamp(c.LobApexMin, minLobApex, maxLobApex)
	c.LobApexMax = clamp(c.LobApexMax, minLobApex, maxLobApex)
	if c.LobApexMax < c.LobApexMin {
		c.LobApexMax = c.LobApexMin
	}
	c.DriveNetClearance = clamp(c.DriveNetClearance, 0, maxNetClearance)

	c.TargetWallMargin = clamp(c.TargetWallMargin, 0, maxTargetWallMargin)
	c.TargetMinFromNet = clamp(c.TargetMinFromNet, 0, maxTargetMinFromNet)
	if maxAbs := CourtHalfLength - c.TargetWallMargin; c.TargetMinFromNet > maxAbs {
		c.TargetMinFromNet = maxAbs
	}

	c.QualPerfectMax = clamp(c.QualPerfectMax, 0, 1)
	c.QualGoodMax = clamp(c.QualGoodMax, 0, 1)
	if c.QualGoodMax < c.QualPerfectMax {
		c.QualGoodMax = c.QualPerfectMax
	}
	c.DevPowerMax = clamp(c.DevPowerMax, 0, maxRestDelta)
}

// sanitizeSpinAndMisc bounds §10's spin deltas, PointPauseTime and the
// legacy v1 kick fields.
func (c *Config) sanitizeSpinAndMisc() {
	c.SpinFloorFriction = clamp(c.SpinFloorFriction, -maxSpinDelta, maxSpinDelta)
	c.SpinFloorRest = clamp(c.SpinFloorRest, -maxSpinDelta, maxSpinDelta)
	c.SpinWallRest = clamp(c.SpinWallRest, -maxSpinDelta, maxSpinDelta)
	c.SpinWallDrop = clamp(c.SpinWallDrop, 0, maxSpinWallDrop)
	c.SpinDecay = clamp(c.SpinDecay, 0, 1)

	c.PointPauseTime = clamp(c.PointPauseTime, 0, maxDurationSec)

	c.KickReach = clamp(c.KickReach, 0, maxKickReach)
	c.KickMinSpeed = clamp(c.KickMinSpeed, 0, maxKickSpeed)
	c.KickMaxSpeed = clamp(c.KickMaxSpeed, 0, maxKickSpeed)
	if c.KickMaxSpeed < c.KickMinSpeed {
		c.KickMaxSpeed = c.KickMinSpeed
	}

	if c.Mode > ModePlayground {
		c.Mode = Mode1v1
	}

	c.DashSpeed = clamp(c.DashSpeed, 0, maxDashSpeed)
	c.DashTime = clamp(c.DashTime, 0, maxDurationSec)
	c.DashStaminaCost = clamp(c.DashStaminaCost, 0, 1)
	c.DashCooldownTime = clamp(c.DashCooldownTime, 0, maxDurationSec)

	if c.ScoringMode > ScoringPadel {
		c.ScoringMode = ScoringSimple
	}
	if c.GamesPerSet < minGamesPerSet {
		c.GamesPerSet = minGamesPerSet
	} else if c.GamesPerSet > maxGamesPerSet {
		c.GamesPerSet = maxGamesPerSet
	}
	if c.SetsToWin < minSetsToWin {
		c.SetsToWin = minSetsToWin
	} else if c.SetsToWin > maxSetsToWin {
		c.SetsToWin = maxSetsToWin
	}
	if c.FirstServer != 0 && c.FirstServer != 1 {
		c.FirstServer = 0
	}
}

// sanitizeParts bounds every §6 PartSpec field and keeps each part's own
// HMin<=HIdeal<=HMax and SpeedMin<=SpeedMax/DevBase<=DevMax/SpinMin<=SpinMax
// invariants intact.
func (c *Config) sanitizeParts() {
	for i := range c.Parts {
		p := &c.Parts[i]
		p.HMin = clamp(p.HMin, 0, maxPartHeight)
		p.HMax = clamp(p.HMax, 0, maxPartHeight)
		if p.HMax < p.HMin {
			p.HMax = p.HMin
		}
		p.HIdeal = clamp(p.HIdeal, p.HMin, p.HMax)

		p.Reach = clamp(p.Reach, 0, maxPartReach)

		p.SpeedMin = clamp(p.SpeedMin, 0, maxPartSpeed)
		p.SpeedMax = clamp(p.SpeedMax, 0, maxPartSpeed)
		if p.SpeedMax < p.SpeedMin {
			p.SpeedMax = p.SpeedMin
		}

		p.DevBase = clamp(p.DevBase, 0, maxPartDev)
		p.DevMax = clamp(p.DevMax, 0, maxPartDev)
		if p.DevMax < p.DevBase {
			p.DevMax = p.DevBase
		}

		p.SpinMin = clamp(p.SpinMin, -1, 1)
		p.SpinMax = clamp(p.SpinMax, -1, 1)
		if p.SpinMax < p.SpinMin {
			p.SpinMax = p.SpinMin
		}
	}
}

// recomputeTickCaches converts every already-clamped *Time duration to a
// cached tick count, with a defense-in-depth clampTickCount at each
// assignment: even though the durations above are always >= 0 and small by
// the time we get here, a future change to the clamps above (or a bug in
// one of them) must never be able to produce a negative int that silently
// wraps into a huge value the moment movement.go/body.go cast it to an
// unsigned tick counter.
func (c *Config) recomputeTickCaches() {
	c.stLockTicks = clampTickCount(ticksOf(c.StaminaLockTime), math.MaxUint16)
	c.chargeMaxTicks = clampTickCount(ticksOf(c.ChargeMaxTime), math.MaxUint16)
	c.bufTicks = clampTickCount(ticksOf(c.StrikeBufferTime), math.MaxUint8)
	c.graceTicks = clampTickCount(ticksOf(c.BodyGraceTime), math.MaxInt32)
	c.serveTimeLimitTicks = clampTickCount(ticksOf(c.ServeTimeLimit), math.MaxInt32)
	c.dashTicks = clampTickCount(ticksOf(c.DashTime), math.MaxUint16)
	c.dashCooldownTicks = clampTickCount(ticksOf(c.DashCooldownTime), math.MaxUint16)
	c.feedIntervalTicks = clampTickCount(ticksOf(c.FeedInterval), math.MaxInt32)
}

// defaultParts returns the §6 contact table. PartNone is left zero-valued
// (it is a sentinel, never indexed for a real contact).
func defaultParts() [PartCount]PartSpec {
	return [PartCount]PartSpec{
		PartFoot: {
			HMin: 0.00, HIdeal: 0.20, HMax: 0.45, Reach: 1.00,
			SpeedMin: 10, SpeedMax: 26,
			DevBase: 0.15, DevMax: 1.6,
			SpinMin: -1, SpinMax: 1,
			AllowLob: true,
		},
		PartKnee: {
			HMin: 0.35, HIdeal: 0.60, HMax: 0.95, Reach: 0.85,
			SpeedMin: 8, SpeedMax: 18,
			DevBase: 0.25, DevMax: 2.0,
			SpinMin: -0.5, SpinMax: 0.5,
			AllowLob: true,
		},
		PartChest: {
			HMin: 0.95, HIdeal: 1.20, HMax: 1.40, Reach: 0.60,
			SpeedMin: 5, SpeedMax: 11,
			DevBase: 0.40, DevMax: 2.2,
			SpinMin: 0, SpinMax: 0,
			AllowLob: true,
		},
		PartShoulder: {
			HMin: 1.30, HIdeal: 1.50, HMax: 1.65, Reach: 0.70,
			SpeedMin: 6, SpeedMax: 14,
			DevBase: 0.35, DevMax: 2.4,
			SpinMin: 0, SpinMax: 0,
			AllowLob: false,
		},
		PartHead: {
			HMin: 1.60, HIdeal: 1.85, HMax: 2.10, Reach: 0.55,
			SpeedMin: 7, SpeedMax: 17,
			DevBase: 0.20, DevMax: 1.8,
			SpinMin: -0.5, SpinMax: 0,
			AllowLob: true,
		},
		PartBicycle: {
			HMin: 1.70, HIdeal: 2.10, HMax: 2.50, Reach: 0.80,
			SpeedMin: 16, SpeedMax: 30,
			DevBase: 0.30, DevMax: 2.8,
			SpinMin: 1, SpinMax: 1,
			AllowLob: false,
		},
	}
}
