import { EventKind, type SimEvent } from "../snapshot";

export interface StepRule {
  /** Matching events needed to complete the step. */
  readonly count: number;
  readonly match: (e: SimEvent) => boolean;
  /** When set, a match only counts after this event, since the last machine feed. */
  readonly armedBy?: (e: SimEvent) => boolean;
}

/** Step cursor over the sim event stream. onEvent returns true when the visible state changed. */
export class TutorialProgress {
  private stepIdx = 0;
  private hitCount = 0;
  private armed = false;

  constructor(private readonly steps: readonly StepRule[]) {}

  get step(): number { return this.stepIdx; }
  get hits(): number { return this.hitCount; }
  get done(): boolean { return this.stepIdx >= this.steps.length; }

  reset(): void {
    this.stepIdx = 0;
    this.hitCount = 0;
    this.armed = false;
  }

  onEvent(e: SimEvent): boolean {
    const s = this.steps[this.stepIdx];
    if (!s) return false;
    if (e.kind === EventKind.Feed) this.armed = false;
    if (s.armedBy?.(e)) {
      this.armed = true;
      return false;
    }
    if (!s.match(e) || (s.armedBy && !this.armed)) return false;
    this.armed = false;
    this.hitCount++;
    if (this.hitCount >= s.count) {
      this.stepIdx++;
      this.hitCount = 0;
    }
    return true;
  }
}
