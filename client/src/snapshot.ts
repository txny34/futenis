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
} as const;

export const SNAP_LEN = 53;
export const MAX_PLAYERS = 4;

/** Offsets inside a player's extras block (starts at P{i}VelX). */
export const Extra = { VelX: 0, VelZ: 1, Stamina: 2, Charge: 3, OnGround: 4, Buffer: 5, Flags: 6 } as const;

/** Snapshot index of player i's X (Y and Z follow). */
export const PLAYER_POS: readonly number[] = [Snap.P0X, Snap.P1X, Snap.P2X, Snap.P3X];
/** Snapshot index of player i's extras block. */
export const PLAYER_EXTRA: readonly number[] = [Snap.P0VelX, Snap.P1VelX, Snap.P2VelX, Snap.P3VelX];

export const FLAG_LOB = 1;
export const FLAG_STAMINA_LOCK = 2;

/** Team of player i (sim teamPlayers: 2v2 -> {0,1},{2,3}; 1v1 -> index = team). */
export function teamOf(i: number, numPlayers: number): number {
  return numPlayers >= MAX_PLAYERS ? i >> 1 : i;
}

// Mirrors sim.Phase.
export const Phase = { Serve: 0, Rally: 1, Point: 2, GameOver: 3 } as const;

// Mirrors sim.Mode.
export const Mode = { OneVsOne: 0, TwoVsTwo: 1, Playground: 2 } as const;

// Mirrors sim.EventKind (§20.2). Old values never change; new ones only appended.
export const EventKind = {
  BounceFloor: 1, HitWall: 2, HitNet: 3, Kick: 4, Point: 5, GameOver: 6, Out: 7,
  Whiff: 8, Jump: 9, BodyHit: 10, Toss: 11, ServeFault: 12, Let: 13,
} as const;

// Mirrors sim.BodyPart. Index 0 = PartNone.
export const PART_NAMES = ["none", "foot", "knee", "chest", "shoulder", "head", "bicycle"] as const;
export type PartName = (typeof PART_NAMES)[number];

// Mirrors sim.Quality.
export const Quality = { Perfect: 0, Good: 1, Forced: 2 } as const;

// Mirrors sim Btn* bits (sim/config.go).
export const Btn = { Strike: 1, Lob: 2, Jump: 4, Sprint: 8 } as const;

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
