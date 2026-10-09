import { assertLayout } from "./layout-check";
import { EVENT_STRIDE, MAX_EVENTS, MAX_PLAYERS, SNAP_LEN, SimEvent } from "./snapshot";

/** Held-level input of one slot (PlayerInput v2). */
export interface HumanInput {
  moveX: number;
  moveZ: number;
  targetX: number;
  targetZ: number;
  buttons: number;
  spin: number;
}

/** Views into engine-owned buffers. Reused every frame: copy if you need to keep data. */
export interface StepResult {
  readonly prev: Float64Array;
  readonly curr: Float64Array;
  readonly events: readonly SimEvent[]; // pool of MAX_EVENTS; only [0, eventCount) are valid
  eventCount: number;
}

export interface MatchSetup {
  /** 0=1v1 1=2v2 2=playground 3=lobby (lobby.md §1). */
  mode: 0 | 1 | 2 | 3;
  difficulty: number;
  pointsToWin: number;
  humanMask: number;
}

const WASM_URL = "dist/futenis.wasm";
export const INPUT_PER_PLAYER = 6; // [moveX, moveZ, targetX, targetZ, buttons, spin]
const INPUT_LEN = MAX_PLAYERS * INPUT_PER_PLAYER; // 24
export const EVENT_COUNT_OFFSET = 2 * SNAP_LEN;
export const EVENTS_OFFSET = EVENT_COUNT_OFFSET + 1;
// OUT_LEN/CFG_LEN derive from the same constants layout() asserts against at
// load time, so a future append on either side fails loudly instead of
// silently misaligning prev/curr snapshot slices (see assertLayout).
export const OUT_LEN = EVENTS_OFFSET + MAX_EVENTS * EVENT_STRIDE;
export const MIN_STEPS = 1;
export const MAX_STEPS = 15; // wasm maxSteps per call
/** Mirrors sim.CfgLen (sim/config_layout.go): 75 scalars (training.md appended
 * 17 at 55..71, lobby.md 3 at 72..74) + the 77-float parts table. */
export const CFG_LEN = 152;
const F64 = 8;

export const emptyEvent = (): SimEvent => ({
  kind: 0, tick: 0, x: 0, y: 0, z: 0, side: 0, player: -1, part: 0, quality: 0, vx: 0, vy: 0, vz: 0, spin: 0,
});

/** What GameLoop needs from a simulation source (local wasm Engine or net/RemoteEngine). */
export interface StepEngine {
  readonly loaded: boolean;
  peek(): StepResult;
  setInput(slot: number, a: HumanInput): void;
  step(n: number): StepResult;
}

/** Decodes the event block of an out buffer into pool and sets result.eventCount. Allocation-free. */
export function readEventsInto(o: Float64Array, pool: readonly SimEvent[], result: StepResult): void {
  const count = Math.max(0, Math.min(MAX_EVENTS, o[EVENT_COUNT_OFFSET] | 0));
  for (let i = 0; i < count; i++) {
    const b = EVENTS_OFFSET + i * EVENT_STRIDE;
    const e = pool[i];
    e.kind = o[b]; e.tick = o[b + 1];
    e.x = o[b + 2]; e.y = o[b + 3]; e.z = o[b + 4];
    e.side = o[b + 5]; e.player = o[b + 6]; e.part = o[b + 7]; e.quality = o[b + 8];
    e.vx = o[b + 9]; e.vy = o[b + 10]; e.vz = o[b + 11]; e.spin = o[b + 12];
  }
  result.eventCount = count;
}

// Engine wraps the WASM bridge. It is the ONLY module aware of the in/out buffer layout.
export class Engine implements StepEngine {
  private readonly input = new Float64Array(INPUT_LEN);
  private readonly out = new Float64Array(OUT_LEN);
  private readonly cfg = new Float64Array(CFG_LEN);
  private readonly result: StepResult;
  private readonly eventPool: SimEvent[];
  private bridge: FutenisBridge | null = null;

  constructor() {
    this.eventPool = Array.from({ length: MAX_EVENTS }, emptyEvent);
    this.result = {
      prev: this.out.subarray(0, SNAP_LEN),
      curr: this.out.subarray(SNAP_LEN, 2 * SNAP_LEN),
      events: this.eventPool,
      eventCount: 0,
    };
  }

  get loaded(): boolean { return this.bridge !== null; }

  async load(): Promise<void> {
    const go = new Go();
    const { instance } = await WebAssembly.instantiateStreaming(fetch(WASM_URL), go.importObject);
    void go.run(instance);
    const bridge = globalThis.futenis;
    if (!bridge) throw new Error("WASM no registró el objeto global 'futenis'");
    assertLayout(bridge, { snapLen: SNAP_LEN, inputPerPlayer: INPUT_PER_PLAYER, eventStride: EVENT_STRIDE, maxEvents: MAX_EVENTS, cfgLen: CFG_LEN });
    check("init", bridge.init(new Uint8Array(this.input.buffer), new Uint8Array(this.out.buffer)));
    this.bridge = bridge;
  }

  /** Writes slot's held-level input. Slots not written stay zero (AI fills them inside wasm). */
  setInput(slot: number, a: HumanInput): void {
    const o = slot * INPUT_PER_PLAYER;
    const b = this.input;
    b[o] = a.moveX;
    b[o + 1] = a.moveZ;
    b[o + 2] = a.targetX;
    b[o + 3] = a.targetZ;
    b[o + 4] = a.buttons;
    b[o + 5] = a.spin;
  }

  clearInputs(): void { this.input.fill(0); }

  step(n: number): StepResult {
    const bridge = this.requireBridge();
    check("step", bridge.step(Math.max(MIN_STEPS, Math.min(MAX_STEPS, Math.floor(n)))));
    readEventsInto(this.out, this.eventPool, this.result);
    return this.result;
  }

  /** Current snapshot pair without stepping. */
  peek(): StepResult {
    this.result.eventCount = 0;
    return this.result;
  }

  reset(): void { this.requireBridge().reset(); }

  configure(s: MatchSetup): void {
    check("configure", this.requireBridge().configure(s.mode, s.difficulty, s.pointsToWin, s.humanMask));
  }

  /** Playground: asks the ball machine for a feed now (wasm feed(), training.md §4). */
  respawnBall(): void {
    const b = this.requireBridge();
    if (b.feed) b.feed();
    else b.respawnBall();
  }

  /** Reads the live Config into an engine-owned buffer (dev panel and match setup, never per frame). */
  getConfig(): Float64Array {
    const u8 = new Uint8Array(this.cfg.buffer);
    const r = this.requireBridge().getConfig(u8);
    if (typeof r === "string" && r !== "") throw new Error(`futenis.getConfig: ${r}`);
    if (r instanceof Uint8Array) u8.set(r.subarray(0, Math.min(r.length, CFG_LEN * F64)));
    return this.cfg;
  }

  setConfig(cfg: Float64Array): void {
    const r = this.requireBridge().setConfig(new Uint8Array(cfg.buffer, cfg.byteOffset, CFG_LEN * F64));
    if (typeof r === "string" && r !== "") throw new Error(`futenis.setConfig: ${r}`);
  }

  private requireBridge(): FutenisBridge {
    if (!this.bridge) throw new Error("Engine no cargado: llamá load() primero");
    return this.bridge;
  }

}

function check(fn: string, err: string | null | undefined): void {
  if (err) throw new Error(`futenis.${fn}: ${err}`);
}
