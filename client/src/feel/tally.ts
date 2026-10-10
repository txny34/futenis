import { EventKind, SimEvent } from "../snapshot";

/** Final tally of a scored match (reused object). Games are the last set's,
 * counted from EvGameWon: the sim zeroes State.Games when a set closes. */
export interface MatchResult { winner: number; games0: number; games1: number }

/** Builds the MatchResult from the event stream. Allocation-free. */
export class MatchTally {
  readonly result: MatchResult = { winner: -1, games0: 0, games1: 0 };
  private readonly games = new Int32Array(2);

  reset(): void {
    this.games.fill(0);
    this.result.winner = -1;
  }

  onEvent(e: SimEvent): void {
    const side = e.side === 1 ? 1 : 0;
    if (e.kind === EventKind.GameWon) this.games[side]++;
    if (e.kind === EventKind.SetWon || e.kind === EventKind.MatchWon || e.kind === EventKind.GameOver) {
      this.result.games0 = this.games[0];
      this.result.games1 = this.games[1];
      if (e.kind !== EventKind.SetWon) this.result.winner = side;
      else this.games.fill(0);
    }
  }
}
