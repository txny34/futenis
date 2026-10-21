import { test } from "node:test";
import assert from "node:assert/strict";
import { TapeAssembler } from "../src/net/tape-chunks";
import { HEADER, Msg, PROTO_VERSION } from "../src/net/protocol";

function chunk(i: number, n: number, data: number[]): Uint8Array {
  const b = new Uint8Array(HEADER + data.length);
  b[0] = Msg.Tape; b[1] = PROTO_VERSION;
  new DataView(b.buffer).setUint16(2, i, true);
  new DataView(b.buffer).setUint16(4, n, true);
  b.set(data, HEADER);
  return b;
}

test("joins ordered chunks", () => {
  const a = new TapeAssembler();
  assert.equal(a.push(chunk(0, 3, [1, 2])), null);
  assert.equal(a.push(chunk(1, 3, [3])), null);
  assert.deepEqual([...(a.push(chunk(2, 3, [4, 5])) ?? [])], [1, 2, 3, 4, 5]);
});

test("out of order or inconsistent chunks reset", () => {
  const a = new TapeAssembler();
  a.push(chunk(0, 2, [1]));
  assert.equal(a.push(chunk(0, 3, [9])), null); // a new tape restarts
  assert.equal(a.push(chunk(2, 3, [9])), null); // skipped index 1: reset
  assert.equal(a.push(chunk(1, 3, [9])), null); // nothing started
  assert.equal(a.push(chunk(5, 3, [9])), null);
  assert.equal(a.push(new Uint8Array(3)), null);
});

test("single chunk tape", () => {
  const a = new TapeAssembler();
  assert.deepEqual([...(a.push(chunk(0, 1, [7])) ?? [])], [7]);
});
