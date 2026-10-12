// Mirrors sim/snapshot.go + wasm/layout.go (contract v2 frozen, contact-v2.md §20).
// CHANGE TOGETHER: any layout change in Go must be applied here in the same commit.
export const Snap = {
  Tick: 0, Phase: 1, Server: 2, Score0: 3, Score1: 4,
  BallX: 5, BallY: 6, BallZ: 7,
  P0X: 8, P0Y: 9, P0Z: 10, P1X: 11, P1Y: 12, P1Z: 13,
  LastTeam: 14,
  BallSpin: 15, BallTossed: 16, NumPlayers: 17, Mode: 18,
  P0VelX: 19, P1VelX: 26,
  P2X: 33, P2VelX: 36,
  P3X: 43, P3VelX: 46,
  // Dash addition (docs/design/contact-v2.md "Dash"): appended at the end,
  // nothing above this line moves. One DashActive flag per player.
  P0Dash: 53, P1Dash: 54, P2Dash: 55, P3Dash: 56,
  // Scoring (contact-v2 §20.8/§21). The client reads Score0/1 (points of the set) and Sets0/1;
  // Points0/1 (0/15/30/40 codes) and Games0/1 are padel-only and stay for the frozen layout.
  Points0: 57, Points1: 58, Games0: 59, Games1: 60, Sets0: 61, Sets1: 62,
  ServerPlayer: 63, RallyShots: 64,
  // Prediction (contact-v2 §20.9/§22): ball's next floor contact...
  LandingX: 65, LandingZ: 66, LandingTicks: 67, LandingOk: 68,
  // ...and per-player predicted contact blocks (see Contact offsets).
  P0Contact: 69, P1Contact: 73, P2Contact: 77, P3Contact: 81,
  // Entrenamiento (docs/design/training.md §3), appended before SnapLen.
  // FeedIn = seconds to the next machine feed, -1 = waiting for demand / ball live.
  Streak: 85, BestStreak: 86, FeedCount: 87, FeedIn: 88, MachineX: 89, MachineZ: 90,
  // Lobby (docs/design/lobby.md §5): active pad (-1 none, 0..2) and hold progress 0..1.
  LobbyPad: 91, LobbyHold: 92,
  // Tiebreak flag (contact-v2 §20.11): padel-only, unused by the client; frozen layout.
  Tiebreak: 93,
} as const;

export const SNAP_LEN = 94;
export const MAX_PLAYERS = 4;

/** Offsets inside a player's extras block (starts at P{i}VelX). */
export const Extra = { VelX: 0, VelZ: 1, Stamina: 2, Charge: 3, OnGround: 4, Buffer: 5, Flags: 6 } as const;

/** Snapshot index of player i's X (Y and Z follow). */
export const PLAYER_POS: readonly number[] = [Snap.P0X, Snap.P1X, Snap.P2X, Snap.P3X];
/** Snapshot index of player i's extras block. */
export const PLAYER_EXTRA: readonly number[] = [Snap.P0VelX, Snap.P1VelX, Snap.P2VelX, Snap.P3VelX];
/** Snapshot index of player i's DashActive flag (docs/design/contact-v2.md "Dash"). */
export const PLAYER_DASH: readonly number[] = [Snap.P0Dash, Snap.P1Dash, Snap.P2Dash, Snap.P3Dash];

/** Offsets inside a player's predicted-contact block (starts at P{i}Contact). */
export const Contact = { Ticks: 0, Part: 1, Quality: 2, Ok: 3 } as const;
/** Snapshot index of player i's predicted-contact block. */
export const PLAYER_CONTACT: readonly number[] = [Snap.P0Contact, Snap.P1Contact, Snap.P2Contact, Snap.P3Contact];

export const FLAG_LOB = 1;
export const FLAG_STAMINA_LOCK = 2;
/** 2v2: this player's next touch can be a pass (sim PassReady): Globo becomes Pase. */
export const FLAG_PASS_READY = 4;

/** Team of player i (sim teamPlayers: 2v2 -> {0,1},{2,3}; 1v1 -> index = team). */
export function teamOf(i: number, numPlayers: number): number {
  return numPlayers >= MAX_PLAYERS ? i >> 1 : i;
}

// Mirrors sim.Phase.
export const Phase = { Serve: 0, Rally: 1, Point: 2, GameOver: 3 } as const;

// Mirrors sim.Mode.
export const Mode = { OneVsOne: 0, TwoVsTwo: 1, Playground: 2, Lobby: 3 } as const;

// Mirrors sim.EventKind (§20.2). Old values never change; new ones only appended.
export const EventKind = {
  BounceFloor: 1, HitWall: 2, HitNet: 3, Kick: 4, Point: 5, GameOver: 6, Out: 7,
  Whiff: 8, Jump: 9, BodyHit: 10, Toss: 11, ServeFault: 12, Let: 13,
  // Dash addition (docs/design/contact-v2.md "Dash"), appended after Let.
  Dash: 14,
  // Scoring (contact-v2 §20.8). SetWon is used; the rest are padel-only (ScoringPadel):
  // frozen ids, the client ignores them.
  GamePoint: 15, BreakPoint: 16, GameWon: 17, SetWon: 18, Ace: 19, MatchWon: 20,
  // Entrenamiento (training.md §2), only in Mode.Playground.
  Feed: 21, ReturnOK: 22, ReturnFail: 23,
  // Lobby (lobby.md §4), only in Mode.Lobby: side = pad index, pos = pad center.
  PadEnter: 24, PadLeave: 25, PadStart: 26,
  // Simple scoring (1v1/2v2): side would win the match with the next point.
  MatchPoint: 27,
  // Best of 3 sets: side would win the set (not yet the match) with the next point.
  SetPoint: 28,
} as const;

// Mirrors sim.BodyPart. Index 0 = PartNone.
export const PART_NAMES = ["none", "foot", "knee", "chest", "shoulder", "head", "bicycle"] as const;

// Mirrors sim.Quality.
export const Quality = { Perfect: 0, Good: 1, Forced: 2 } as const;

// Mirrors sim Btn* bits (sim/config.go). Dash appended last, same as sim's
// iota chain (docs/design/contact-v2.md "Dash").
export const Btn = { Strike: 1, Lob: 2, Jump: 4, Sprint: 8, Dash: 16 } as const;

export const MAX_EVENTS = 32;
// [kind, tick, posX, posY, posZ, side, player, part, quality, velX, velY, velZ, spin, _pad]
export const EVENT_STRIDE = 14;

export interface SimEvent {
  kind: number;
  tick: number;
  x: number;
  y: number;
  z: number;
  /** Team 0/1. */
  side: number;
  /** Index in State.Players, or -1. */
  player: number;
  part: number;
  quality: number;
  vx: number;
  vy: number;
  vz: number;
  spin: number;
}
