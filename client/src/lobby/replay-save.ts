import type { HumanInput } from "../engine";
import { saveReplay, type ReplayMeta } from "../replay/store";
import { setTapeSeat, TapeWriter } from "../replay/tape-format";
import type { MatchOptions } from "../ui/menu";
import { DIFFICULTY_ID, type App } from "./flow";

// Records local bot matches as tape v2 and saves every finished 1v1/2v2 match
// (bot or online) to the replay store, keyed by its history record's `at`.
const MAX_TAPE_TICKS = 20 * 60 * 120; // 20 min at 120 Hz; longer matches keep no replay
const ZERO: Readonly<HumanInput> = Object.freeze({ moveX: 0, moveZ: 0, targetX: 0, targetZ: 0, buttons: 0, spin: 0 });
// Preallocated slot array: recordStep only swaps slot 0 (no per-frame allocation).
const slots: Readonly<HumanInput>[] = [ZERO, ZERO, ZERO, ZERO];
const LOCAL_MASK = 1; // the single local human on slot 0

// Online: the result (meta) and the server's tape arrive in any order; the second one saves.
let pending: { meta?: ReplayMeta; tape?: Uint8Array } = {};

/** New match (local or online): forget the previous match's replay state. */
export function resetReplayState(app: App): void {
  pending = {};
  app.lastReplay = null;
}

/** Match start: a fresh tape for 1v1/2v2 against bots, none otherwise. Call after configure. */
export function beginLocalTape(app: App, o: MatchOptions): void {
  resetReplayState(app);
  if (o.mode !== "1v1" && o.mode !== "2v2") { app.tape = null; return; }
  app.tape = new TapeWriter({
    mode: o.mode === "2v2" ? 1 : 0, difficulty: DIFFICULTY_ID[o.difficulty], initMask: LOCAL_MASK,
    pointsToWin: o.pointsToWin, seat: 0, firstServer: app.engine.firstServer(),
  }, app.engine.getConfig(), MAX_TAPE_TICKS);
}

/** loop.onStep: n ticks ran with the human's input a. Allocation-free. */
export function recordStep(tape: TapeWriter, n: number, a: Readonly<HumanInput>): void {
  slots[0] = a;
  tape.frame(n, LOCAL_MASK, slots);
}

function store(app: App, meta: ReplayMeta, tape: Uint8Array): void {
  app.lastReplay = { meta, tape };
  void saveReplay(meta, tape);
}

/** A 1v1/2v2 match ended (not a replay): save now (local) or once the server's tape is in (online). */
export function noteResult(app: App, meta: ReplayMeta): void {
  if (!meta.online) {
    const tape = app.tape?.bytes() ?? null;
    if (tape) store(app, meta, tape);
    return;
  }
  if (pending.tape) { store(app, meta, pending.tape); pending = {}; }
  else pending = { meta };
}

/** RemoteEngine.onTape: the server's gunzipped tape, rewritten to view from our seat. */
export function noteTape(app: App, tape: Uint8Array, seat: number): void {
  setTapeSeat(tape, seat);
  if (pending.meta) { store(app, pending.meta, tape); pending = {}; }
  else pending = { tape };
}
