import { test } from "node:test";
import assert from "node:assert/strict";
import { TutorialProgress, type StepRule } from "../src/tutorial/progress";
import { EventKind, type SimEvent } from "../src/snapshot";

const ev = (kind: number, side = 0): SimEvent =>
  ({ kind, tick: 0, x: 0, y: 0, z: 0, side, player: 0, part: 0, quality: 0, vx: 0, vy: 0, vz: 0, spin: 0 });

const RETURN: StepRule = { count: 2, match: (e) => e.kind === EventKind.ReturnOK };
const WALL: StepRule = {
  count: 1, match: (e) => e.kind === EventKind.ReturnOK,
  armedBy: (e) => e.kind === EventKind.HitWall && e.side === 0,
};

test("a step completes after its count of matches", () => {
  const p = new TutorialProgress([RETURN]);
  assert.equal(p.onEvent(ev(EventKind.ReturnOK)), true);
  assert.equal(p.hits, 1);
  p.onEvent(ev(EventKind.ReturnOK));
  assert.equal(p.done, true);
});

test("an armed step ignores a return without the wall first", () => {
  const p = new TutorialProgress([WALL]);
  assert.equal(p.onEvent(ev(EventKind.ReturnOK)), false);
  assert.equal(p.onEvent(ev(EventKind.HitWall, 1)), false); // the rival's wall does not arm
  assert.equal(p.onEvent(ev(EventKind.ReturnOK)), false);
  assert.equal(p.done, false);
});

test("own wall then return completes; a new feed disarms", () => {
  const p = new TutorialProgress([WALL]);
  p.onEvent(ev(EventKind.HitWall, 0));
  p.onEvent(ev(EventKind.Feed));
  assert.equal(p.onEvent(ev(EventKind.ReturnOK)), false);
  p.onEvent(ev(EventKind.HitWall, 0));
  assert.equal(p.onEvent(ev(EventKind.ReturnOK)), true);
  assert.equal(p.done, true);
});

test("reset starts over and disarms", () => {
  const p = new TutorialProgress([RETURN, WALL]);
  p.onEvent(ev(EventKind.ReturnOK));
  p.onEvent(ev(EventKind.ReturnOK));
  p.onEvent(ev(EventKind.HitWall, 0));
  p.reset();
  assert.equal(p.step, 0);
  assert.equal(p.hits, 0);
});
