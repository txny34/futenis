import { EventKind, PART_NAMES, SimEvent } from "../snapshot";

// Client-side shot classifier for game-feel callouts. Pure read of the event
// stream (never decides rules): part from EvKick, volley = no floor bounce
// since the previous touch, lob/smash from the launch velocity.
export const ShotKind = { None: 0, Chilena: 1, Cabezazo: 2, Remate: 3, Volea: 4, Globo: 5 } as const;
export type ShotKindId = (typeof ShotKind)[keyof typeof ShotKind];

const REMATE_MIN_Y = 1.5; // m: contact above the head-ish
const REMATE_MIN_SPEED = 15; // m/s
const LOB_MIN_VY = 7; // m/s upward
export const LOB_VY_RATIO = 0.6; // vy relative to horizontal speed

export class ShotFeed {
  private bounces = 0;
  private touches = 0;
  /** True between a remate and the ball's next floor bounce (splat spot). */
  rematePending = false;

  /** Feed every event in order; returns the shot kind for kicks, else None. */
  onEvent(e: SimEvent): ShotKindId {
    switch (e.kind) {
      case EventKind.BounceFloor:
        this.bounces++;
        return ShotKind.None;
      case EventKind.Point:
      case EventKind.ServeFault:
      case EventKind.Let:
        this.reset();
        return ShotKind.None;
      case EventKind.BodyHit:
        this.touch();
        return ShotKind.None;
      case EventKind.Kick:
        return this.kick(e);
      default:
        return ShotKind.None;
    }
  }

  /** New rally / new match. */
  reset(): void {
    this.touches = 0;
    this.bounces = 0;
    this.rematePending = false;
  }

  private touch(): void {
    this.touches++;
    this.bounces = 0;
    this.rematePending = false;
  }

  private kick(e: SimEvent): ShotKindId {
    const volley = this.touches > 0 && this.bounces === 0;
    this.touch();
    const kind = classify(e, volley);
    this.rematePending = kind === ShotKind.Remate;
    return kind;
  }
}

function classify(e: SimEvent, volley: boolean): ShotKindId {
  const part = PART_NAMES[e.part] ?? "none";
  if (part === "bicycle") return ShotKind.Chilena;
  if (part === "head") return ShotKind.Cabezazo;
  const horiz = Math.hypot(e.vx, e.vz);
  const speed = Math.hypot(horiz, e.vy);
  if (e.y > REMATE_MIN_Y && e.vy < 0 && speed > REMATE_MIN_SPEED) return ShotKind.Remate;
  if (volley) return ShotKind.Volea;
  if (isLob(e)) return ShotKind.Globo;
  return ShotKind.None;
}

/** High, mostly vertical launch (globo). Shared with the tutorial. */
export function isLob(e: SimEvent): boolean {
  return e.vy > LOB_MIN_VY && e.vy > Math.hypot(e.vx, e.vz) * LOB_VY_RATIO;
}
