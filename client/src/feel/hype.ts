import { EventKind, SimEvent } from "../snapshot";

// Crowd/announcer excitement from the event stream (never decides rules):
// touches in the current rally and consecutive points per team.
export const LONG_RALLY = 8; // touches: a point after this many is a "puntazo"
export const STREAK_CALL = 3; // consecutive points that earn a callout

export interface HypeMoment {
  /** Touches of the rally that just ended. */
  touches: number;
  /** Consecutive points of the winner, including this one. */
  streak: number;
  winner: number;
}

export class Hype {
  private touches = 0;
  private lastWinner = -1;
  private streak = 0;
  private readonly moment: HypeMoment = { touches: 0, streak: 0, winner: 0 };

  /** 0..1: how far the live rally is toward a long one. */
  get rally01(): number { return Math.min(1, this.touches / LONG_RALLY); }

  reset(): void {
    this.touches = 0;
    this.lastWinner = -1;
    this.streak = 0;
  }

  /** Returns the (reused) moment when a point closes a long rally or extends a streak, else null. */
  onEvent(e: SimEvent): Readonly<HypeMoment> | null {
    switch (e.kind) {
      case EventKind.Kick: // body grazes are skipped: the sim merges graze + kick into one touch
        this.touches++;
        return null;
      case EventKind.ServeFault:
      case EventKind.Let:
        this.touches = 0;
        return null;
      case EventKind.Point:
        return this.point(e.side === 1 ? 1 : 0);
      default:
        return null;
    }
  }

  private point(winner: number): Readonly<HypeMoment> | null {
    this.streak = winner === this.lastWinner ? this.streak + 1 : 1;
    this.lastWinner = winner;
    const touches = this.touches;
    this.touches = 0;
    if (touches < LONG_RALLY && this.streak < STREAK_CALL) return null;
    this.moment.touches = touches;
    this.moment.streak = this.streak;
    this.moment.winner = winner;
    return this.moment;
  }
}
