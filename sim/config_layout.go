package sim

// Live-tuning wire format for Config (docs/design/contact-v2.md §13).
// CfgLen/CfgScalarLen/CfgPartsStart below describe a flat []float64 the
// client reads/writes between frames (never mid-Step). Scalars come first,
// in declaration order; the body-part table (Config.Parts) is flattened
// after them, one block of cfgPartFieldCount floats per part, in BodyPart
// order (PartNone included, even though it is never populated, to keep
// index math trivial).

// cfgPartFieldCount is how many floats describe one PartSpec: HMin, HIdeal,
// HMax, Reach, SpeedMin, SpeedMax, DevBase, DevMax, SpinMin, SpinMax,
// AllowLob (0/1).
const cfgPartFieldCount = 11

// Scalar field indices, in the same order Config declares them. Keeping
// this list explicit (rather than reflection) keeps WriteConfig/ReadConfig
// allocation-free and panic-free on malformed input.
const (
	cfgGravity = iota
	cfgFloorRestitution
	cfgWallRestitution
	cfgFloorFriction
	cfgAirDrag

	cfgPlayerSpeed
	cfgSprintSpeed
	cfgPlayerAccel
	cfgPlayerAirAccel
	cfgPlayerGravity
	cfgJumpSpeed

	cfgStaminaDrain
	cfgStaminaRegen
	cfgStaminaLockTime
	cfgJumpStaminaCost

	cfgChargeMaxTime
	cfgChargeMoveFactor
	cfgStrikeBufferTime

	cfgBodyRestitution
	cfgBodyGraceTime

	cfgServeTossHeight
	cfgServeTossOffset
	cfgServeTossSpeed
	cfgServiceLineDist

	cfgMomentumFactor
	cfgMomentumMax

	cfgLobApexMin
	cfgLobApexMax
	cfgDriveNetClearance
	cfgTargetMinFromNet
	cfgTargetWallMargin

	cfgQualPerfectMax
	cfgQualGoodMax
	cfgDevPowerMax

	cfgSpinFloorFriction
	cfgSpinFloorRest
	cfgSpinWallRest
	cfgSpinWallDrop
	cfgSpinDecay

	cfgPointPauseTime

	cfgKickReach
	cfgKickMinSpeed
	cfgKickMaxSpeed

	cfgPointsToWin
	cfgMode

	// CfgScalarLen is the number of scalar (non-Parts) tunable floats.
	CfgScalarLen
)

// CfgPartsStart is the offset of the first PartSpec field in the flattened
// layout, right after the scalars.
const CfgPartsStart = CfgScalarLen

// CfgLen is the total float count WriteConfig writes / ReadConfig expects:
// every scalar field plus the whole flattened Parts table.
const CfgLen = CfgScalarLen + int(PartCount)*cfgPartFieldCount

// WriteConfig fills dst (len(dst) >= CfgLen) with c's tunable fields,
// without allocating.
func WriteConfig(c *Config, dst []float64) {
	dst[cfgGravity] = c.Gravity
	dst[cfgFloorRestitution] = c.FloorRestitution
	dst[cfgWallRestitution] = c.WallRestitution
	dst[cfgFloorFriction] = c.FloorFriction
	dst[cfgAirDrag] = c.AirDrag

	dst[cfgPlayerSpeed] = c.PlayerSpeed
	dst[cfgSprintSpeed] = c.SprintSpeed
	dst[cfgPlayerAccel] = c.PlayerAccel
	dst[cfgPlayerAirAccel] = c.PlayerAirAccel
	dst[cfgPlayerGravity] = c.PlayerGravity
	dst[cfgJumpSpeed] = c.JumpSpeed

	dst[cfgStaminaDrain] = c.StaminaDrain
	dst[cfgStaminaRegen] = c.StaminaRegen
	dst[cfgStaminaLockTime] = c.StaminaLockTime
	dst[cfgJumpStaminaCost] = c.JumpStaminaCost

	dst[cfgChargeMaxTime] = c.ChargeMaxTime
	dst[cfgChargeMoveFactor] = c.ChargeMoveFactor
	dst[cfgStrikeBufferTime] = c.StrikeBufferTime

	dst[cfgBodyRestitution] = c.BodyRestitution
	dst[cfgBodyGraceTime] = c.BodyGraceTime

	dst[cfgServeTossHeight] = c.ServeTossHeight
	dst[cfgServeTossOffset] = c.ServeTossOffset
	dst[cfgServeTossSpeed] = c.ServeTossSpeed
	dst[cfgServiceLineDist] = c.ServiceLineDist

	dst[cfgMomentumFactor] = c.MomentumFactor
	dst[cfgMomentumMax] = c.MomentumMax

	dst[cfgLobApexMin] = c.LobApexMin
	dst[cfgLobApexMax] = c.LobApexMax
	dst[cfgDriveNetClearance] = c.DriveNetClearance
	dst[cfgTargetMinFromNet] = c.TargetMinFromNet
	dst[cfgTargetWallMargin] = c.TargetWallMargin

	dst[cfgQualPerfectMax] = c.QualPerfectMax
	dst[cfgQualGoodMax] = c.QualGoodMax
	dst[cfgDevPowerMax] = c.DevPowerMax

	dst[cfgSpinFloorFriction] = c.SpinFloorFriction
	dst[cfgSpinFloorRest] = c.SpinFloorRest
	dst[cfgSpinWallRest] = c.SpinWallRest
	dst[cfgSpinWallDrop] = c.SpinWallDrop
	dst[cfgSpinDecay] = c.SpinDecay

	dst[cfgPointPauseTime] = c.PointPauseTime

	dst[cfgKickReach] = c.KickReach
	dst[cfgKickMinSpeed] = c.KickMinSpeed
	dst[cfgKickMaxSpeed] = c.KickMaxSpeed

	dst[cfgPointsToWin] = float64(c.PointsToWin)
	dst[cfgMode] = float64(c.Mode)

	writePartsBlock(c, dst[CfgPartsStart:])
}

// writePartsBlock flattens c.Parts into dst (len(dst) >= PartCount*cfgPartFieldCount).
func writePartsBlock(c *Config, dst []float64) {
	for i, ps := range c.Parts {
		o := i * cfgPartFieldCount
		dst[o+0] = ps.HMin
		dst[o+1] = ps.HIdeal
		dst[o+2] = ps.HMax
		dst[o+3] = ps.Reach
		dst[o+4] = ps.SpeedMin
		dst[o+5] = ps.SpeedMax
		dst[o+6] = ps.DevBase
		dst[o+7] = ps.DevMax
		dst[o+8] = ps.SpinMin
		dst[o+9] = ps.SpinMax
		dst[o+10] = boolToFloat(ps.AllowLob)
	}
}

// ReadConfig builds a Config from src (len(src) >= CfgLen), then Sanitizes
// it (clamps + recomputes the cached tick fields) before returning.
func ReadConfig(src []float64) Config {
	var c Config
	c.Gravity = src[cfgGravity]
	c.FloorRestitution = src[cfgFloorRestitution]
	c.WallRestitution = src[cfgWallRestitution]
	c.FloorFriction = src[cfgFloorFriction]
	c.AirDrag = src[cfgAirDrag]

	c.PlayerSpeed = src[cfgPlayerSpeed]
	c.SprintSpeed = src[cfgSprintSpeed]
	c.PlayerAccel = src[cfgPlayerAccel]
	c.PlayerAirAccel = src[cfgPlayerAirAccel]
	c.PlayerGravity = src[cfgPlayerGravity]
	c.JumpSpeed = src[cfgJumpSpeed]

	c.StaminaDrain = src[cfgStaminaDrain]
	c.StaminaRegen = src[cfgStaminaRegen]
	c.StaminaLockTime = src[cfgStaminaLockTime]
	c.JumpStaminaCost = src[cfgJumpStaminaCost]

	c.ChargeMaxTime = src[cfgChargeMaxTime]
	c.ChargeMoveFactor = src[cfgChargeMoveFactor]
	c.StrikeBufferTime = src[cfgStrikeBufferTime]

	c.BodyRestitution = src[cfgBodyRestitution]
	c.BodyGraceTime = src[cfgBodyGraceTime]

	c.ServeTossHeight = src[cfgServeTossHeight]
	c.ServeTossOffset = src[cfgServeTossOffset]
	c.ServeTossSpeed = src[cfgServeTossSpeed]
	c.ServiceLineDist = src[cfgServiceLineDist]

	c.MomentumFactor = src[cfgMomentumFactor]
	c.MomentumMax = src[cfgMomentumMax]

	c.LobApexMin = src[cfgLobApexMin]
	c.LobApexMax = src[cfgLobApexMax]
	c.DriveNetClearance = src[cfgDriveNetClearance]
	c.TargetMinFromNet = src[cfgTargetMinFromNet]
	c.TargetWallMargin = src[cfgTargetWallMargin]

	c.QualPerfectMax = src[cfgQualPerfectMax]
	c.QualGoodMax = src[cfgQualGoodMax]
	c.DevPowerMax = src[cfgDevPowerMax]

	c.SpinFloorFriction = src[cfgSpinFloorFriction]
	c.SpinFloorRest = src[cfgSpinFloorRest]
	c.SpinWallRest = src[cfgSpinWallRest]
	c.SpinWallDrop = src[cfgSpinWallDrop]
	c.SpinDecay = src[cfgSpinDecay]

	c.PointPauseTime = src[cfgPointPauseTime]

	c.KickReach = src[cfgKickReach]
	c.KickMinSpeed = src[cfgKickMinSpeed]
	c.KickMaxSpeed = src[cfgKickMaxSpeed]

	c.PointsToWin = int(src[cfgPointsToWin])
	c.Mode = Mode(src[cfgMode])

	c.Parts = readPartsBlock(src[CfgPartsStart:])
	c.Sanitize()
	return c
}

// readPartsBlock is writePartsBlock's inverse.
func readPartsBlock(src []float64) [PartCount]PartSpec {
	var parts [PartCount]PartSpec
	for i := range parts {
		o := i * cfgPartFieldCount
		parts[i] = PartSpec{
			HMin:     src[o+0],
			HIdeal:   src[o+1],
			HMax:     src[o+2],
			Reach:    src[o+3],
			SpeedMin: src[o+4],
			SpeedMax: src[o+5],
			DevBase:  src[o+6],
			DevMax:   src[o+7],
			SpinMin:  src[o+8],
			SpinMax:  src[o+9],
			AllowLob: src[o+10] != 0,
		}
	}
	return parts
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// Sanitize clamps Config to physically sane bounds and recomputes the
// cached derived tick counts (docs/design/contact-v2.md §1.1, §13). Called
// by DefaultConfig and ReadConfig; must also be called after any live edit
// from the dev panel, between frames (never mid-Step).
func (c *Config) Sanitize() {
	c.FloorRestitution = clamp(c.FloorRestitution, 0, 1)
	c.WallRestitution = clamp(c.WallRestitution, 0, 1)
	c.BodyRestitution = clamp(c.BodyRestitution, 0, 1)
	c.ChargeMoveFactor = clamp(c.ChargeMoveFactor, 0, 1)
	c.DevPowerMax = clamp(c.DevPowerMax, 0, 1)

	if c.PlayerSpeed < 0 {
		c.PlayerSpeed = 0
	}
	if c.SprintSpeed < 0 {
		c.SprintSpeed = 0
	}
	if c.PointsToWin < 1 {
		c.PointsToWin = 1
	}

	for i := range c.Parts {
		p := &c.Parts[i]
		if p.HIdeal < p.HMin {
			p.HIdeal = p.HMin
		}
		if p.HIdeal > p.HMax {
			p.HIdeal = p.HMax
		}
	}

	c.stLockTicks = ticksOf(c.StaminaLockTime)
	c.chargeMaxTicks = ticksOf(c.ChargeMaxTime)
	c.bufTicks = ticksOf(c.StrikeBufferTime)
	c.graceTicks = ticksOf(c.BodyGraceTime)
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
