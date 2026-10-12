import { EventKind, SimEvent } from "../snapshot";

/** Final tally of a scored match (reused object): the winner; games0/1 are
 * kept for the stored history's shape and filled with the sets by the result
 * screen, which reads them from the snapshot. */
export interface MatchResult { winner: number; games0: number; games1: number }

/** Builds the MatchResult from the event stream. Allocation-free. */
export class MatchTally {
  readonly result: MatchResult = { winner: -1, games0: 0, games1: 0 };
  private readonly points = new Int32Array(2);

  reset(): void {
    this.points.fill(0);
    this.result.winner = -1;
  }

  onEvent(e: SimEvent): void {
    const side = e.side === 1 ? 1 : 0;
    if (e.kind === EventKind.Point) this.points[side]++;
    if (e.kind === EventKind.GameOver) {
      this.result.games0 = this.points[0];
      this.result.games1 = this.points[1];
      this.result.winner = side;
    }
  }
}
