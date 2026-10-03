package sim

// TickRate is the fixed simulation frequency (Hz). Never step with a variable dt.
const TickRate = 60

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

// Config holds tunable physics parameters. Part of the match state so it is deterministic.
type Config struct {
	Gravity          float64 // m/s², positive number, applied on -Y
	FloorRestitution float64 // 0..1
	WallRestitution  float64 // 0..1
	FloorFriction    float64 // fraction of horizontal speed kept per floor bounce
	AirDrag          float64 // linear drag coefficient per second
	PlayerSpeed      float64 // m/s
	KickReach        float64 // max distance player center -> ball to allow a kick
	KickMinSpeed     float64 // ball speed at Power=0
	KickMaxSpeed     float64 // ball speed at Power=1
	PointsToWin      int
}

// DefaultConfig returns the baseline tuned Config used by a fresh Match.
func DefaultConfig() Config {
	return Config{
		Gravity:          9.81,
		FloorRestitution: 0.75,
		WallRestitution:  0.7,
		FloorFriction:    0.85,
		AirDrag:          0.1,
		PlayerSpeed:      6.0,
		KickReach:        1.0,
		KickMinSpeed:     8.0,
		KickMaxSpeed:     22.0,
		PointsToWin:      11,
	}
}

// PlayerInput is an action-level input for one tick. Device mapping lives in the client.
type PlayerInput struct {
	MoveX, MoveZ float64 // desired direction, each -1..1 (normalized by sim)
	Kick         bool    // edge-triggered by client: true only on the press tick
	AimX, AimZ   float64 // horizontal aim direction (normalized by sim); zero = straight to the other side
	Loft         float64 // 0..1, vertical component of the kick
	Power        float64 // 0..1
}

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
}

// Player is one player's kinematic state and which half of the court they
// are confined to.
type Player struct {
	Pos, Vel Vec3
	Side     int8 // 0 = X negative, 1 = X positive
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
// action. Order matters for wire compatibility: EvOut was added last.
const (
	EvBounceFloor EventKind = iota + 1 // ball bounced on the floor
	EvHitWall                          // ball hit a side/back wall
	EvHitNet                           // ball clipped the net below NetHeight
	EvKick                             // a player kicked the ball
	EvPoint                            // a point was awarded
	EvGameOver                         // the match ended
	EvOut                              // ball left the court bounds
)

// Event is emitted by Step. Side meaning depends on Kind:
// floor/wall: side of the court where it happened; kick: kicker; point/gameover: winner.
type Event struct {
	Kind EventKind
	Tick uint32
	Pos  Vec3
	Side int8
}

// RallyState is the bookkeeping the rules need inside a rally.
type RallyState struct {
	LastKicker    int8   // -1 if none yet
	BouncesOnSide [2]int // floor bounces on each side since the last kick
	PauseTicks    int    // remaining ticks in PhasePoint
}

// State is the full deterministic game state. Copyable by value (snapshots, rollback).
type State struct {
	Tick    uint32
	Phase   Phase
	Server  int8 // side that serves next / is serving
	Score   [2]int
	Ball    Ball
	Players [2]Player
	Rally   RallyState
	Cfg     Config
}
