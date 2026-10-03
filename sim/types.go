package sim

import "math"

// TickRate is the fixed simulation frequency (Hz). Never step with a
// variable dt. Contact v2 (docs/design/contact-v2.md §1.1) moves this from
// 60 to 120 Hz to match Rocket League-style physics; every tick-derived
// constant in this package is computed from TickRate (via ticksOf, see
// config_layout.go) so changing it again never silently breaks tuning.
const TickRate = 120

// Dt is the fixed timestep in seconds.
const Dt = 1.0 / TickRate

// Court geometry (meters). Padel court: 20 x 10, net at X=0.
const (
	CourtHalfLength = 10.0
	CourtHalfWidth  = 5.0
	WallHeight      = 3.0
	NetHeight       = 0.88
	BallRadius      = 0.11
	PlayerRadius    = 0.35
	PlayerHeight    = 1.8
)

// MaxPlayers is the fixed array size of State.Players: padel is 2v2, so this
// covers every supported Mode (1v1 uses slots 0/1 only; State.NumPlayers
// says how many of the MaxPlayers slots are active). Fixed-size (not a
// slice) so State stays comparable with == and zero-alloc to copy.
const MaxPlayers = 4

// Mode selects how many players are active and whether points are scored
// (docs/design/contact-v2.md "Modos y 2v2").
type Mode uint8

const (
	Mode1v1        Mode = iota // 2 players, 1 per team (today's default)
	Mode2v2                    // 4 players, 2 per team (real padel)
	ModePlayground             // free-form sandbox: no scoring, ball can be respawned on demand
)

// numPlayersForMode returns how many of State.Players are active for mode.
func numPlayersForMode(mode Mode) int8 {
	if mode == Mode2v2 {
		return 4
	}
	return 2
}

// ticksOf converts a duration in seconds to a tick count at TickRate,
// rounding to the nearest tick. Config.Sanitize uses this to cache derived
// tick counts (see Config's unexported *Ticks fields) so Step never repeats
// the conversion.
func ticksOf(sec float64) int {
	return int(math.Round(sec * TickRate))
}

// PartSpec describes one contact body part's reach, power and precision
// (docs/design/contact-v2.md §6). Values are tunable in vivo via Config.
type PartSpec struct {
	HMin, HIdeal, HMax float64 // relative ball height band this part can reach (m)
	Reach              float64 // max horizontal distance ball<->player (m)
	SpeedMin, SpeedMax float64 // shot speed range this part can produce (m/s)
	DevBase, DevMax    float64 // target deviation radius at u=0 / u=1 (m)
	SpinMin, SpinMax   float64 // spin this part can impart, clamps PlayerInput.Spin
	AllowLob           bool    // false => a Lob request downgrades to Drive (§7.2)
}

// BodyPart identifies which part of the player's body made contact with the
// ball. PartNone means no part is in range (whiff).
type BodyPart uint8

const (
	PartNone BodyPart = iota
	PartFoot
	PartKnee
	PartChest
	PartShoulder
	PartHead
	PartBicycle
	PartCount
)

// ShotType is latched from the button that started the charge (press of
// BtnStrike -> ShotDrive, BtnLob -> ShotLob) and used by the solver (§9).
type ShotType uint8

const (
	ShotDrive ShotType = iota
	ShotLob
)

// Quality is how well-timed/positioned a contact was, derived from the
// normalized error `u` (§8): QualPerfect -> no deviation, QualForced -> the
// full DevMax/DevPowerMax range applies.
type Quality uint8

const (
	QualPerfect Quality = iota
	QualGood
	QualForced
)

// Ball is the ball's kinematic state: position and velocity in meters/(m/s).
type Ball struct {
	Pos, Vel Vec3
	// Resting is true once the ball has settled on the floor (a downward
	// contact slow enough that Vel.Y was clamped to 0 instead of bounced).
	// Edge-triggered: resolveFloor emits EvBounceFloor exactly once when
	// this flips false->true, not on every tick while the ball keeps
	// resting/rolling. Cleared once the ball is airborne again.
	Resting bool
	// Out is true once the ball has left the court bounds. Edge-triggered:
	// resolveWalls emits EvOut exactly once when this flips false->true, not
	// on every tick while the ball stays outside. Cleared once the ball is
	// back inside the court bounds.
	Out bool
	// Spin is -1..1 (- back, + top), constant in flight (no Magnus, §1
	// non-objetivos), decayed by Config.SpinDecay on every bounce (§10).
	//
	// TODO(wave1): wired into resolveFloor/reflectWalls (spin.go).
	Spin float64
	// Tossed is true once the ball has been released/dropped for the
	// current serve attempt (docs/design/contact-v2.md §7.5 — real padel
	// serve adapted to feet; this replaces the original self-toss design).
	// Before Tossed, the ball is held at rest in front of the server.
	//
	// TODO(wave1): set on BtnStrike press during PhaseServe; drives the
	// serve bounce/box judging state machine in rules.go.
	Tossed bool
}

// Player is one player's kinematic state and which half of the court (and
// which side of that half, in 2v2) they belong to.
type Player struct {
	Pos, Vel Vec3 // Vel.Y becomes meaningful once wave1 wires jump/gravity
	Team     int8 // 0 = X negative half, 1 = X positive half (was "Side" pre-2v2)
	Slot     int8 // 0 = right partner, 1 = left partner (2v2 only; always 0 in 1v1). Starting-position/serve-box reference only; wave0 does not confine movement by Slot.
	OnGround bool // TODO(wave1): jump (player.go); Wave 0 never sets this false

	// Held is in.Buttons from the previous tick, used to detect press/
	// release edges without the sim ever receiving edge-triggered input.
	Held uint8

	Stamina     float64  // 0..1, boost-model resource (§5)
	StaminaLock uint16   // ticks remaining without regen after hitting 0
	ChargeTicks uint16   // ticks Strike/Lob has been held, saturates at Config's cached chargeMaxTicks
	ShotType    ShotType // latched on press: ShotDrive | ShotLob
	Buffer      uint8    // ticks remaining of strike intention (0 = none)
	BufCharge   float64  // charge 0..1, frozen at release
	BufTarget   Vec3     // destination frozen at release (Y = BallRadius)
	BufSpin     int8     // spin frozen at release
	StruckAt    uint32   // tick of this player's last contact (body-collision grace, §7.4)
}

// Phase is which stage of a point the match is in.
type Phase uint8

// Phase values, in the order a point normally progresses through them.
const (
	PhaseServe    Phase = iota // ball waiting for the server's kick
	PhaseRally                 // ball in play
	PhasePoint                 // point just scored, short pause before next serve
	PhaseGameOver              // match decided, Step is a no-op
)

// EventKind identifies what kind of Event was emitted by Step.
type EventKind uint8

// EventKind values. BounceFloor/HitWall/HitNet/Out describe something that
// happened to the ball; Kick/Point/GameOver describe a player or score
// action. Order matters for wire compatibility: existing values never
// change meaning, new kinds are only ever appended at the end.
const (
	EvBounceFloor EventKind = iota + 1 // ball bounced on the floor
	EvHitWall                          // ball hit a side/back wall
	EvHitNet                           // ball clipped the net below NetHeight
	EvKick                             // a player kicked the ball
	EvPoint                            // a point was awarded
	EvGameOver                         // the match ended
	EvOut                              // ball left the court bounds

	// Contract v2 additions (docs/design/contact-v2.md §11.1), appended
	// after EvOut so existing numeric values never change:
	EvWhiff      // a strike buffer expired with no contact
	EvJump       // a player jumped
	EvBodyHit    // the ball bounced off a player's body capsule (counts as a touch, §7.4)
	EvToss       // the server released/dropped the ball for a serve attempt (§7.5)
	EvServeFault // a serve attempt failed (out of the diagonal box, net-no-cross, wall before bounce, or a miss) — docs/design/contact-v2.md §7.5
	EvLet        // the serve touched the net but still landed in the correct box; the attempt repeats without counting as a fault
)

// Event is emitted by Step. Side meaning depends on Kind:
// floor/wall: side (team) of the court where it happened; kick: kicker's
// team; point/gameover: winning team; servefault/let: the serving team.
// Player disambiguates which of State.Players caused the event (-1 when
// not applicable, e.g. a bare floor/wall bounce with no player involved).
type Event struct {
	Kind    EventKind
	Tick    uint32
	Pos     Vec3
	Side    int8     // team (0/1), not a State.Players array index
	Player  int8     // State.Players index of the player involved, or -1
	Part    BodyPart // only EvKick/EvBodyHit
	Quality Quality  // only EvKick
	Vel     Vec3     // EvKick: ball velocity post-contact (network + feedback)
	Spin    float64  // EvKick
}

// RallyState is the bookkeeping the rules need inside a rally. One touch
// per TEAM per crossing: either partner may return the ball in 2v2, but
// only one touch total counts (docs/design/contact-v2.md "Modos y 2v2").
type RallyState struct {
	LastTeam      int8   // -1 if none yet; the team (not player) that last legally touched the ball
	LastPlayer    int8   // -1 if none yet; which of State.Players (within LastTeam) actually touched it
	BouncesOnSide [2]int // floor bounces on each side since the last kick

	PauseTicks int // remaining ticks in PhasePoint

	// ServerPlayer is which of State.Players is the one individual allowed
	// to serve this point (serverPlayerIndex, frozen at resetForServe).
	ServerPlayer int8

	// Serve state machine (docs/design/contact-v2.md §7.5 — real padel
	// serve adapted to feet, overriding the original self-toss design).
	//
	// TODO(wave1): resetForServe/rules.go wire these; Wave 0 only reserves
	// the fields so the contract is frozen.
	ServeAttempt uint8 // 1 = first serve, 2 = second serve (after a fault)
	ServeBox     int8  // 0 = right diagonal box, 1 = left diagonal box; alternates every point (serveBoxForScore)
}

// serveBoxForScore returns which diagonal service box the server must serve
// from/into, per real padel's right/left alternation: the box flips every
// point played, independent of who is serving.
//
// TODO(wave1): resetForServe calls this to set RallyState.ServeBox, and the
// serve judge in rules.go uses it to validate where the serve landed.
func serveBoxForScore(score [2]int) int8 {
	total := score[0] + score[1]
	return int8(total % 2)
}

// serverPlayerIndex returns which of State.Players is allowed to serve for
// team, alternating between partners each time that team serves again
// (docs/design/contact-v2.md "Modos y 2v2": chosen serve-order rule for
// wave 0 — NOT real padel's full round-robin across all 4 players turn by
// turn across games, which needs a game/set model this codebase doesn't
// have yet; this is the simplest rule that still gives both partners the
// serve). In 1v1 there is exactly one player per team, so it is always
// that player.
//
// teamPlayers(team) fixes the array-index convention: team 0 = {0,1}, team
// 1 = {2,3} (slot 0 = right, slot 1 = left).
func serverPlayerIndex(s *State, team int8) int8 {
	if s.NumPlayers < 4 {
		return team
	}
	a, b := teamPlayers(team)
	if s.LastServerPlayer[team] == a {
		return b
	}
	return a
}

// teamPlayers returns the two State.Players array indices that belong to
// team (2v2 convention: team 0 = {0,1}, team 1 = {2,3}).
func teamPlayers(team int8) (a, b int8) {
	return team * 2, team*2 + 1
}

// State is the full deterministic game state. Copyable by value (snapshots, rollback).
type State struct {
	Tick       uint32
	Phase      Phase
	Server     int8 // team that serves next / is serving
	NumPlayers int8 // 2 (Mode1v1/ModePlayground) or 4 (Mode2v2); how many of Players are active
	Score      [2]int
	Ball       Ball
	Players    [MaxPlayers]Player

	// LastServerPlayer remembers, per team, which Players index served last
	// time that team served, so serverPlayerIndex can alternate partners in
	// 2v2. -1 until that team has served at least once.
	LastServerPlayer [2]int8

	Rally RallyState
	Cfg   Config
}
