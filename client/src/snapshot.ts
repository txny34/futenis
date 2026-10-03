// Mirrors sim/snapshot.go. CHANGE TOGETHER: any layout change in Go must be applied here in the same commit.
export const Snap = {
  Tick: 0,
  Phase: 1,
  Server: 2,
  Score0: 3,
  Score1: 4,
  BallX: 5,
  BallY: 6,
  BallZ: 7,
  P0X: 8,
  P0Y: 9,
  P0Z: 10,
  P1X: 11,
  P1Y: 12,
  P1Z: 13,
  LastKicker: 14,
} as const;

export const SNAP_LEN = 15;

// Mirrors sim.Phase.
export const Phase = { Serve: 0, Rally: 1, Point: 2, GameOver: 3 } as const;

// Mirrors sim.EventKind.
export const EventKind = {
  BounceFloor: 1,
  HitWall: 2,
  HitNet: 3,
  Kick: 4,
  Point: 5,
  GameOver: 6,
  Out: 7,
} as const;

export const MAX_EVENTS = 32;
export const EVENT_STRIDE = 6; // [kind, tick, x, y, z, side]

export interface SimEvent {
  kind: number;
  tick: number;
  x: number;
  y: number;
  z: number;
  side: number;
}
