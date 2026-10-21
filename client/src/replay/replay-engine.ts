import { emptyEvent, type HumanInput, MAX_STEPS, type StepEngine, type StepResult } from "../engine";
import { MAX_EVENTS, MAX_PLAYERS, type SimEvent } from "../snapshot";
import { type DecodedTape, tapeInput } from "./tape-format";

// Plays a tape v2 through the real wasm engine (deterministic re-simulation).
// Records are split into ≤ MAX_STEPS calls (held inputs: same result). seek()
// rewinds (fresh session from the tape's setup) when going back, then runs
// the sim silently up to the target. Speed lives in the loop (timeScale).

/** The wasm Engine surface a replay drives. */
export interface ReplaySource extends StepEngine {
  setHumanMask(mask: number): void;
}

function copyEvent(dst: SimEvent, s: Readonly<SimEvent>): void {
  dst.kind = s.kind; dst.tick = s.tick; dst.x = s.x; dst.y = s.y; dst.z = s.z; dst.side = s.side;
  dst.player = s.player; dst.part = s.part; dst.quality = s.quality;
  dst.vx = s.vx; dst.vy = s.vy; dst.vz = s.vz; dst.spin = s.spin;
}

export class ReplayEngine implements StepEngine {
  private r = 0; // current record
  private left = 0; // ticks left in record r (0 = not started)
  private at = 0; // ticks played
  private mask: number;
  private readonly input: HumanInput = { moveX: 0, moveZ: 0, targetX: 0, targetZ: 0, buttons: 0, spin: 0 };
  private readonly pool: SimEvent[] = Array.from({ length: MAX_EVENTS }, emptyEvent);
  private readonly result: StepResult;

  /** rewind: puts inner back at the tape's start (configureFirst + setConfig + reset + view). Already done for the first play. */
  constructor(private readonly inner: ReplaySource, private readonly tape: DecodedTape, private readonly rewind: () => void) {
    const r = inner.peek();
    this.result = { prev: r.prev, curr: r.curr, events: this.pool, eventCount: 0 };
    this.mask = tape.header.initMask;
  }

  get loaded(): boolean { return this.inner.loaded; }
  get tick(): number { return this.at; }
  get total(): number { return this.tape.totalTicks; }
  get ended(): boolean { return this.at >= this.tape.totalTicks; }

  peek(): StepResult { this.result.eventCount = 0; return this.result; }

  setInput(): void { /* the tape drives every slot */ }

  step(n: number): StepResult {
    this.result.eventCount = 0;
    this.advance(Math.max(0, Math.floor(n)), true);
    return this.result;
  }

  /** Jumps to tick t (clamped). Silent: no events reach the loop. */
  seek(t: number): void {
    const target = Math.max(0, Math.min(this.tape.totalTicks, Math.floor(t)));
    if (target < this.at) {
      this.rewind();
      this.r = 0; this.left = 0; this.at = 0;
      this.mask = this.tape.header.initMask;
    }
    this.advance(target - this.at, false);
    this.result.eventCount = 0;
  }

  private advance(k: number, collect: boolean): void {
    const t = this.tape;
    while (k > 0 && this.r < t.count) {
      if (this.left === 0) {
        this.begin(this.r);
        if (this.left === 0) { this.r++; continue; }
      }
      const c = Math.min(this.left, k, MAX_STEPS);
      const res = this.inner.step(c);
      if (collect) this.merge(res);
      this.left -= c; k -= c; this.at += c;
      if (this.left === 0) this.r++;
    }
  }

  private begin(r: number): void {
    const t = this.tape;
    if (t.masks[r] !== this.mask) {
      this.mask = t.masks[r];
      this.inner.setHumanMask(this.mask);
    }
    for (let s = 0; s < MAX_PLAYERS; s++) {
      tapeInput(t, r, s, this.input);
      this.inner.setInput(s, this.input);
    }
    this.left = t.ticks[r];
  }

  /** prev/curr alias the engine's buffers; only events need merging. */
  private merge(res: StepResult): void {
    let c = this.result.eventCount;
    for (let k = 0; k < res.eventCount && c < MAX_EVENTS; k++) copyEvent(this.pool[c++], res.events[k]);
    this.result.eventCount = c;
  }
}
