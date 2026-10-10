import { emptyEvent, type HumanInput, MAX_STEPS, type StepEngine, type StepResult } from "../engine";
import { HUMAN_SLOT } from "../loop";
import { MAX_EVENTS, type SimEvent } from "../snapshot";
import type { Tape } from "./tape";

// Plays a Tape through the real wasm engine. Each recorded step call is
// replayed as one unit (same n, same input → same sim), paced by a tick debt
// so playback runs at real speed whatever the display rate. Events of several
// records in one frame are merged. Live input is ignored.
const MAX_DEBT = 2 * MAX_STEPS;

function copyEvent(dst: SimEvent, s: Readonly<SimEvent>): void {
  dst.kind = s.kind; dst.tick = s.tick; dst.x = s.x; dst.y = s.y; dst.z = s.z; dst.side = s.side;
  dst.player = s.player; dst.part = s.part; dst.quality = s.quality;
  dst.vx = s.vx; dst.vy = s.vy; dst.vz = s.vz; dst.spin = s.spin;
}

export class ReplayEngine implements StepEngine {
  private i = 0;
  private debt = 0;
  private readonly input: HumanInput = { moveX: 0, moveZ: 0, targetX: 0, targetZ: 0, buttons: 0, spin: 0 };
  private readonly pool: SimEvent[] = Array.from({ length: MAX_EVENTS }, emptyEvent);
  private readonly result: StepResult;

  constructor(private readonly inner: StepEngine, private readonly tape: Tape) {
    const r = inner.peek();
    this.result = { prev: r.prev, curr: r.curr, events: this.pool, eventCount: 0 };
  }

  get loaded(): boolean { return this.inner.loaded; }

  peek(): StepResult { this.result.eventCount = 0; return this.result; }

  setInput(): void { /* the tape drives slot 0 */ }

  step(n: number): StepResult {
    this.result.eventCount = 0;
    this.debt = Math.min(MAX_DEBT, this.debt + n);
    while (this.i < this.tape.length && this.tape.ticks(this.i) <= this.debt) {
      const ticks = this.tape.read(this.i++, this.input);
      this.debt -= ticks;
      this.inner.setInput(HUMAN_SLOT, this.input);
      this.merge(this.inner.step(ticks));
    }
    return this.result;
  }

  /** prev/curr already alias the engine's buffers; only the events need merging. */
  private merge(r: StepResult): void {
    let c = this.result.eventCount;
    for (let k = 0; k < r.eventCount && c < MAX_EVENTS; k++) copyEvent(this.pool[c++], r.events[k]);
    this.result.eventCount = c;
  }
}
