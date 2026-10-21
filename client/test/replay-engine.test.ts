import { test } from "node:test";
import assert from "node:assert/strict";
import { CFG_LEN, emptyEvent, type HumanInput, type StepResult } from "../src/engine";
import { ReplayEngine, type ReplaySource } from "../src/replay/replay-engine";
import { decodeTape, TapeWriter } from "../src/replay/tape-format";
import { MAX_EVENTS } from "../src/snapshot";

class FakeSource implements ReplaySource {
  readonly loaded = true;
  calls: number[] = [];
  masks: number[] = [];
  inputs: number[] = []; // slot 0 moveX seen at each step
  private x = 0;
  private readonly res: StepResult = { prev: new Float64Array(4), curr: new Float64Array(4), events: Array.from({ length: MAX_EVENTS }, emptyEvent), eventCount: 0 };
  peek(): StepResult { return this.res; }
  setInput(slot: number, a: HumanInput): void { if (slot === 0) this.x = a.moveX; }
  setHumanMask(m: number): void { this.masks.push(m); }
  step(n: number): StepResult { this.calls.push(n); this.inputs.push(this.x); this.res.eventCount = 1; return this.res; }
}

const zero = (): HumanInput => ({ moveX: 0, moveZ: 0, targetX: 0, targetZ: 0, buttons: 0, spin: 0 });

function tape(): ReturnType<typeof decodeTape> {
  const w = new TapeWriter({ mode: 0, difficulty: 1, initMask: 0b11, pointsToWin: 5, seat: 0, firstServer: 0 }, new Float64Array(CFG_LEN), 1 << 20);
  const ins = [zero(), zero(), zero(), zero()];
  w.frame(40, 0b11, ins);            // record 0: 40 ticks
  ins[0] = { ...zero(), moveX: 1 };
  w.frame(10, 0b11, ins);            // record 1: 10 ticks, slot 0 moves
  w.frame(10, 0b01, ins);            // record 2: mask change (slot 1 left)
  const b = w.bytes();
  assert.ok(b);
  return decodeTape(b);
}

test("step plays exact ticks in ≤ 15-tick calls with the tape's inputs", () => {
  const src = new FakeSource();
  const e = new ReplayEngine(src, tape(), () => assert.fail("no rewind on play"));
  e.step(45);
  assert.equal(e.tick, 45);
  assert.ok(src.calls.every((c) => c <= 15));
  assert.equal(src.calls.reduce((a, b) => a + b, 0), 45);
  assert.equal(src.inputs[src.inputs.length - 1], 1, "record 1 input applied");
  e.step(100);
  assert.equal(e.tick, 60);
  assert.ok(e.ended);
  assert.deepEqual(src.masks, [0b01]);
});

test("seek back rewinds and replays silently", () => {
  const src = new FakeSource();
  let rewinds = 0;
  const e = new ReplayEngine(src, tape(), () => { rewinds++; });
  e.step(55);
  e.seek(20);
  assert.equal(rewinds, 1);
  assert.equal(e.tick, 20);
  assert.equal(e.peek().eventCount, 0);
  e.seek(58);
  assert.equal(rewinds, 1, "forward seek never rewinds");
  assert.equal(e.tick, 58);
  e.seek(1e9);
  assert.equal(e.tick, 60);
  e.seek(-5);
  assert.equal(e.tick, 0);
});
