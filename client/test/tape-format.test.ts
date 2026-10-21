import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { CFG_LEN, type HumanInput } from "../src/engine";
import { decodeTape, setTapeSeat, tapeInput, TapeWriter } from "../src/replay/tape-format";

const golden = new Uint8Array(readFileSync("game/testdata/tape_v2.bin"));
const zero = (): HumanInput => ({ moveX: 0, moveZ: 0, targetX: 0, targetZ: 0, buttons: 0, spin: 0 });

function goldenScript(): Uint8Array | null {
  const cfgBytes = new Uint8Array(CFG_LEN * 8).map((_, i) => i % 251);
  const cfg = new Float64Array(cfgBytes.buffer);
  const w = new TapeWriter({ mode: 1, difficulty: 2, initMask: 0b0101, pointsToWin: 5, seat: 0, firstServer: 1 }, cfg, 1 << 20);
  const ins = [zero(), zero(), zero(), zero()];
  w.frame(2, 0b0101, ins);
  w.frame(2, 0b0101, ins);
  ins[0] = { moveX: 0.5, moveZ: -1, targetX: 6.25, targetZ: -2.5, buttons: 1, spin: -1 };
  w.frame(3, 0b0101, ins);
  ins[2] = { moveX: -0.75, moveZ: 0, targetX: 4, targetZ: 0, buttons: 9, spin: 1 };
  ins[1] = { ...zero(), moveX: 1 };
  w.frame(2, 0b0101, ins);
  w.frame(2, 0b0001, ins);
  return w.bytes();
}

test("decodes Go's golden tape", () => {
  const t = decodeTape(golden);
  assert.deepEqual(t.header, { mode: 1, difficulty: 2, initMask: 0b0101, pointsToWin: 5, seat: 0, firstServer: 1 });
  assert.equal(new Uint8Array(t.cfg.buffer)[7], 7);
  assert.deepEqual([...t.ticks], [4, 3, 2, 2]);
  assert.equal(t.totalTicks, 11);
  const a = zero();
  tapeInput(t, 1, 0, a);
  assert.equal(a.targetX, 6.25);
  assert.equal(a.spin, -1);
  tapeInput(t, 2, 1, a);
  assert.equal(a.moveX, 0, "a bot slot is never recorded");
  tapeInput(t, 3, 2, a);
  assert.equal(a.buttons, 9);
  assert.equal(t.masks[3], 0b0001);
});

test("the TS writer matches Go byte for byte", () => {
  const mine = goldenScript();
  assert.ok(mine);
  assert.deepEqual([...mine], [...golden]);
});

test("seat stamp and bad input", () => {
  const b = golden.slice();
  setTapeSeat(b, 2);
  assert.equal(decodeTape(b).header.seat, 2);
  assert.throws(() => decodeTape(new Uint8Array(10)));
  assert.throws(() => decodeTape(golden.slice(0, golden.length - 3)));
});

test("a too-long tape gives null", () => {
  const w = new TapeWriter({ mode: 0, difficulty: 1, initMask: 1, pointsToWin: 5, seat: 0, firstServer: 0 }, new Float64Array(CFG_LEN), 10);
  const ins = [zero(), zero(), zero(), zero()];
  for (let i = 0; i < 8; i++) w.frame(2, 1, ins);
  assert.equal(w.bytes(), null);
});
