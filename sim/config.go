package sim

// Config holds tunable physics parameters. Part of the match state so it is
// deterministic and network-replayable. See docs/design/contact-v2.md §3.1
// for units/defaults and the live-tuning contract (§13, config_layout.go).
type Config struct {
	// Ball physics (existing).
	Gravity          float64 // m/s², positive number, applied on -Y
	FloorRestitution float64 // 0..1
	WallRestitution  float64 // 0..1
	FloorFriction    float64 // fraction of horizontal speed kept per floor bounce
	AirDrag          float64 // linear drag coefficient per second
	PointsToWin      int

	// Player movement.
	PlayerSpeed    float64 // m/s, cruise speed (existing)
	SprintSpeed    float64 // m/s, with BtnSprint and Stamina > 0
	PlayerAccel    float64 // m/s², on ground
	PlayerAirAccel float64 // m/s², airborne (reduced control)
	PlayerGravity  float64 // m/s², player's own gravity (snappier than the ball)
	JumpSpeed      float64 // m/s, vertical takeoff speed

	// Stamina (boost model, §5).
	StaminaDrain    float64 // 1/s
	StaminaRegen    float64 // 1/s
	StaminaLockTime float64 // s, no regen right after hitting 0
	JumpStaminaCost float64 // 0..1, consumed per jump

	// Charge / strike buffer (§7.1).
	ChargeMaxTime    float64 // s, full charge
	ChargeMoveFactor float64 // 0..1, speed multiplier while charging
	StrikeBufferTime float64 // s, intention window after release

	// Body collision (§7.4).
	BodyRestitution float64 // 0..1, ball bounce off the player capsule
	BodyGraceTime   float64 // s, striker is immune to their own body right after contact

	// Serve (§7.5 — real padel serve adapted to feet; overrides the
	// original self-toss design, per coordinator decision).
	//
	// ServeTossHeight/ServeTossOffset describe where the ball is released
	// (waist height, in front of the server) before it drops under gravity.
	// ServeTossSpeed is kept for tuning a small upward assist on the drop
	// but defaults to 0 (no self-toss launch; the real-padel drop serve
	// just lets gravity do the work).
	ServeTossHeight float64 // m, release height (waist, ~1.0 m)
	ServeTossOffset float64 // m, in front of the server, toward the net
	ServeTossSpeed  float64 // m/s, optional upward assist on release (default 0)
	ServiceLineDist float64 // m, net -> service line; the serve must bounce once beyond this before being struck (6.95 m in real padel)

	// ServeTimeLimit (code-review hardening, not in docs/design/contact-v2.md
	// §7.5 yet): how long, in seconds, the server may sit in PhaseServe
	// without pressing Strike before the ball auto-drops (tossServe) so a
	// silent/idle/AI-less server can never hang a match forever.
	ServeTimeLimit float64 // s, default 8

	// Momentum (§9).
	MomentumFactor float64 // fraction of player velocity added to a shot
	MomentumMax    float64 // m/s, momentum contribution cap

	// Shot shaping (§8, §9).
	LobApexMin        float64 // m
	LobApexMax        float64 // m
	DriveNetClearance float64 // m, margin over NetHeight+BallRadius for an ideal drive
	TargetMinFromNet  float64 // m, closest a requested target may be to the net
	TargetWallMargin  float64 // m, closest a requested target may be to a wall

	// Quality / deviation thresholds (§8).
	QualPerfectMax float64 // normalized error <= this => QualPerfect
	QualGoodMax    float64 // normalized error <= this => QualGood (else QualForced)
	DevPowerMax    float64 // 0..1, +/- power deviation on a forced contact

	// Spin (§10).
	SpinFloorFriction float64 // horizontal friction delta per unit of spin on a floor bounce
	SpinFloorRest     float64 // vertical restitution delta per unit of spin on a floor bounce
	SpinWallRest      float64 // restitution delta on a wall/glass bounce (full backspin)
	SpinWallDrop      float64 // m/s, downward velocity backspin adds on a wall bounce
	SpinDecay         float64 // spin multiplier applied on every bounce

	PointPauseTime float64 // s, pause after a point before the next serve (180 ticks at 120 Hz)

	// Legacy v1 kick path (KickReach/KickMinSpeed/KickMaxSpeed). Per
	// docs/design/contact-v2.md §3.1 these "se eliminan (pasan a la tabla
	// de partes)" once wave 1's strike.go lands; Wave 0 keeps them so the
	// temporary Buttons-release -> old-kick compat shim in physics.go can
	// still produce a playable prototype.
	KickReach    float64
	KickMinSpeed float64
	KickMaxSpeed float64

	// Parts is the body-part contact table (§6), tunable in vivo.
	Parts [PartCount]PartSpec

	// Mode selects 1v1 / 2v2 / Playground (see MaxPlayers, State.NumPlayers).
	Mode Mode

	// Derived, cached by Sanitize() so Step never recomputes a ticksOf
	// conversion on the hot path. Unexported: not part of the live-tuning
	// wire format (config_layout.go flattens the tunable fields only).
	stLockTicks         int
	chargeMaxTicks      int
	bufTicks            int
	graceTicks          int
	serveTimeLimitTicks int
}

// DefaultConfig returns the baseline tuned Config used by a fresh Match.
func DefaultConfig() Config {
	c := Config{
		Gravity:          9.81,
		FloorRestitution: 0.75,
		WallRestitution:  0.7,
		FloorFriction:    0.85,
		AirDrag:          0.1,
		PointsToWin:      11,

		PlayerSpeed:    6.0,
		SprintSpeed:    8.5,
		PlayerAccel:    40,
		PlayerAirAccel: 12,
		PlayerGravity:  16,
		JumpSpeed:      6.0,

		StaminaDrain:    0.50,
		StaminaRegen:    0.33,
		StaminaLockTime: 0.75,
		JumpStaminaCost: 0.05,

		ChargeMaxTime:    0.9,
		ChargeMoveFactor: 0.8,
		StrikeBufferTime: 0.1,

		BodyRestitution: 0.3,
		BodyGraceTime:   0.1,

		ServeTossHeight: 1.0,
		ServeTossOffset: 0.5,
		ServeTossSpeed:  0,
		ServiceLineDist: 6.95,
		ServeTimeLimit:  8.0,

		MomentumFactor: 0.35,
		MomentumMax:    4.0,

		LobApexMin:        4.0,
		LobApexMax:        7.0,
		DriveNetClearance: 0.25,
		TargetMinFromNet:  1.0,
		TargetWallMargin:  0.3,

		QualPerfectMax: 0.25,
		QualGoodMax:    0.60,
		DevPowerMax:    0.15,

		SpinFloorFriction: 0.25,
		SpinFloorRest:     0.15,
		SpinWallRest:      0.45,
		SpinWallDrop:      1.5,
		SpinDecay:         0.5,

		PointPauseTime: 1.5,

		KickReach:    1.0,
		KickMinSpeed: 8.0,
		KickMaxSpeed: 22.0,

		Parts: defaultParts(),
		Mode:  Mode1v1,
	}
	c.Sanitize()
	return c
}

// PlayerInput is an action-level input for one tick. Device mapping lives in
// the client. v2 (docs/design/contact-v2.md §2): the sim receives held
// levels, never edges; sim detects press/release itself by comparing
// Buttons against Player.Held from the previous tick. This makes step(n)
// with a repeated input correct and inputs idempotent (safe for network
// resend and for the bot).
type PlayerInput struct {
	MoveX, MoveZ     float64 // -1..1, desired direction (sim normalizes if |v|>1)
	TargetX, TargetZ float64 // meters, destination point on the RIVAL half of the court
	Buttons          uint8   // bitmask of held levels, see Btn* below
	Spin             int8    // -1 backspin / 0 flat / +1 topspin (mouse wheel, 3 discrete levels)
}

// Button bits for PlayerInput.Buttons. BtnStrike/BtnLob held = charging;
// released = the shot (or, in the serve's drop rule, the ball release)
// fires. See docs/design/contact-v2.md §2 and §7.5.
const (
	BtnStrike uint8 = 1 << iota // held: charge; release: Drive-type shot. On serve: ball release/drop.
	BtnLob                      // held: charge; release: Lob-type shot.
	BtnJump                     // Space
	BtnSprint                   // Shift
)

// btnMask is every known button bit; sanitizeInput clears unknown bits.
const btnMask = BtnStrike | BtnLob | BtnJump | BtnSprint
