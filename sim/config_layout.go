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

	// cfgServeTimeLimit is appended last (after cfgMode, not grouped with the
	// rest of the Serve fields above) so every existing scalar index keeps
	// its wire meaning -- adding a new tunable here never reshuffles
	// cfgPointsToWin/cfgMode or anything the client/wasm bridge already
	// addresses by position.
	cfgServeTimeLimit

	// Dash additions (docs/design/contact-v2.md "Dash") are appended after
	// cfgServeTimeLimit, the previous end of the layout, so no existing
	// index moves.
	cfgDashSpeed
	cfgDashTime
	cfgDashStaminaCost
	cfgDashCooldownTime

	// Scoring additions (docs/design/contact-v2.md "Puntuación de pádel")
	// are appended after cfgDashCooldownTime, the previous end of the
	// layout, so no existing index moves. GoldenPoint is written as 0/1.
	cfgScoringMode
	cfgGoldenPoint
	cfgGamesPerSet
	cfgSetsToWin
	cfgFirstServer

	// Entrenamiento additions (docs/design/training.md §1) are appended
	// after cfgFirstServer, the previous end of the layout, so no existing
	// index moves. Booleans are written as 0/1.
	cfgMachineZMin
	cfgMachineZMax
	cfgMachineXMin
	cfgMachineXMax
	cfgFeedZMin
	cfgFeedZMax
	cfgFeedXMin
	cfgFeedXMax
	cfgFeedShots
	cfgFeedPowerMin
	cfgFeedPowerMax
	cfgFeedQualityMin
	cfgFeedQualityMax
	cfgFeedOnDemand
	cfgFeedInterval
	cfgFeedVolleyOnly
	cfgFeedValidBounce

	// Lobby additions (docs/design/lobby.md §3), appended after cfgFeedValidBounce.
	cfgLobbyHoldTime
	cfgLobbyHalfX
	cfgLobbyHalfZ

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
	dst[cfgServeTimeLimit] = c.ServeTimeLimit

	dst[cfgDashSpeed] = c.DashSpeed
	dst[cfgDashTime] = c.DashTime
	dst[cfgDashStaminaCost] = c.DashStaminaCost
	dst[cfgDashCooldownTime] = c.DashCooldownTime

	dst[cfgScoringMode] = float64(c.ScoringMode)
	dst[cfgGoldenPoint] = boolToFloat(c.GoldenPoint)
	dst[cfgGamesPerSet] = float64(c.GamesPerSet)
	dst[cfgSetsToWin] = float64(c.SetsToWin)
	dst[cfgFirstServer] = float64(c.FirstServer)

	writeFeedBlock(c, dst)
	writeLobbyBlock(c, dst)
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
	c.ServeTimeLimit = src[cfgServeTimeLimit]

	c.DashSpeed = src[cfgDashSpeed]
	c.DashTime = src[cfgDashTime]
	c.DashStaminaCost = src[cfgDashStaminaCost]
	c.DashCooldownTime = src[cfgDashCooldownTime]

	c.ScoringMode = ScoringMode(src[cfgScoringMode])
	c.GoldenPoint = src[cfgGoldenPoint] != 0
	c.GamesPerSet = int(src[cfgGamesPerSet])
	c.SetsToWin = int(src[cfgSetsToWin])
	c.FirstServer = int8(src[cfgFirstServer])

	readFeedBlock(&c, src)
	readLobbyBlock(&c, src)
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

// writeFeedBlock flattens the Entrenamiento fields (docs/design/training.md §1).
func writeFeedBlock(c *Config, dst []float64) {
	dst[cfgMachineZMin], dst[cfgMachineZMax] = c.MachineZMin, c.MachineZMax
	dst[cfgMachineXMin], dst[cfgMachineXMax] = c.MachineXMin, c.MachineXMax
	dst[cfgFeedZMin], dst[cfgFeedZMax] = c.FeedZMin, c.FeedZMax
	dst[cfgFeedXMin], dst[cfgFeedXMax] = c.FeedXMin, c.FeedXMax
	dst[cfgFeedShots] = float64(c.FeedShots)
	dst[cfgFeedPowerMin], dst[cfgFeedPowerMax] = c.FeedPowerMin, c.FeedPowerMax
	dst[cfgFeedQualityMin], dst[cfgFeedQualityMax] = c.FeedQualityMin, c.FeedQualityMax
	dst[cfgFeedOnDemand] = boolToFloat(c.FeedOnDemand)
	dst[cfgFeedInterval] = c.FeedInterval
	dst[cfgFeedVolleyOnly] = boolToFloat(c.FeedVolleyOnly)
	dst[cfgFeedValidBounce] = float64(c.FeedValidBounce)
}

// readFeedBlock is writeFeedBlock's inverse. Integer fields are clamped to
// their type's range before the cast so a hostile float never wraps;
// Sanitize then applies the documented bounds.
func readFeedBlock(c *Config, src []float64) {
	c.MachineZMin, c.MachineZMax = src[cfgMachineZMin], src[cfgMachineZMax]
	c.MachineXMin, c.MachineXMax = src[cfgMachineXMin], src[cfgMachineXMax]
	c.FeedZMin, c.FeedZMax = src[cfgFeedZMin], src[cfgFeedZMax]
	c.FeedXMin, c.FeedXMax = src[cfgFeedXMin], src[cfgFeedXMax]
	c.FeedShots = uint8(clamp(src[cfgFeedShots], 0, float64(FeedShotAll)))
	c.FeedPowerMin, c.FeedPowerMax = src[cfgFeedPowerMin], src[cfgFeedPowerMax]
	c.FeedQualityMin, c.FeedQualityMax = src[cfgFeedQualityMin], src[cfgFeedQualityMax]
	c.FeedOnDemand = src[cfgFeedOnDemand] != 0
	c.FeedInterval = src[cfgFeedInterval]
	c.FeedVolleyOnly = src[cfgFeedVolleyOnly] != 0
	c.FeedValidBounce = uint8(clamp(src[cfgFeedValidBounce], 0, maxFeedValidBounce))
}
