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

type Ball struct {
	Pos, Vel Vec3
}

type Player struct {
	Pos, Vel Vec3
	Side     int8 // 0 = X negative, 1 = X positive
}

type Phase uint8

const (
	PhaseServe Phase = iota // ball waiting for the server's kick
	PhaseRally              // ball in play
	PhasePoint              // point just scored, short pause before next serve
	PhaseGameOver
)

type EventKind uint8

const (
	EvBounceFloor EventKind = iota + 1
	EvHitWall
	EvHitNet
	EvKick
	EvPoint
	EvGameOver
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
	LastKicker       int8   // -1 if none yet
	BouncesOnSide    [2]int // floor bounces on each side since the last kick
	WallBeforeBounce bool   // ball hit a wall on the receiving side before bouncing there
	PauseTicks       int    // remaining ticks in PhasePoint
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
