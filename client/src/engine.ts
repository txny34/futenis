import { EVENT_STRIDE, MAX_EVENTS, SNAP_LEN, SimEvent } from "./snapshot";

export interface PlayerAction {
  moveX: number;
  moveZ: number;
  kick: boolean;
  aimX: number;
  aimZ: number;
  loft: number;
  power: number;
}

/** Views into engine-owned buffers. Reused every frame: copy if you need to keep data. */
export interface StepResult {
  readonly prev: Float64Array;
  readonly curr: Float64Array;
  readonly events: readonly SimEvent[]; // pool of MAX_EVENTS; only [0, eventCount) are valid
  eventCount: number;
}

const WASM_URL = "dist/futenis.wasm";
const INPUT_STRIDE = 7; // [moveX, moveZ, kick, aimX, aimZ, loft, power]
const EVENT_COUNT_OFFSET = 2 * SNAP_LEN;
const EVENTS_OFFSET = EVENT_COUNT_OFFSET + 1;
const OUT_LEN = EVENTS_OFFSET + MAX_EVENTS * EVENT_STRIDE;
const MIN_STEPS = 1;
const MAX_STEPS = 15;

// Engine wraps the WASM bridge. It is the ONLY module aware of the in/out buffer layout.
export class Engine {
  private readonly input = new Float64Array(2 * INPUT_STRIDE);
  private readonly out = new Float64Array(OUT_LEN);
  private readonly result: StepResult;
  private readonly eventPool: SimEvent[];
  private bridge: FutenisBridge | null = null;

  constructor() {
    this.eventPool = Array.from({ length: MAX_EVENTS }, () => ({ kind: 0, tick: 0, x: 0, y: 0, z: 0, side: 0 }));
    this.result = {
      prev: this.out.subarray(0, SNAP_LEN),
      curr: this.out.subarray(SNAP_LEN, 2 * SNAP_LEN),
      events: this.eventPool,
      eventCount: 0,
    };
  }

  async load(): Promise<void> {
    const go = new Go();
    const { instance } = await WebAssembly.instantiateStreaming(fetch(WASM_URL), go.importObject);
    void go.run(instance);
    const bridge = globalThis.futenis;
    if (!bridge) throw new Error("WASM no registró el objeto global 'futenis'");
    const err = bridge.init(new Uint8Array(this.input.buffer), new Uint8Array(this.out.buffer));
    if (err) throw new Error(`futenis.init: ${err}`);
    this.bridge = bridge;
  }

  step(p0: PlayerAction, p1: PlayerAction, n: number): StepResult {
    const bridge = this.requireBridge();
    this.writeInput(0, p0);
    this.writeInput(INPUT_STRIDE, p1);
    const err = bridge.step(Math.max(MIN_STEPS, Math.min(MAX_STEPS, Math.floor(n))));
    if (err) throw new Error(`futenis.step: ${err}`);
    this.readEvents();
    return this.result;
  }

  /** Current snapshot pair without stepping (first frame). */
  peek(): StepResult {
    this.result.eventCount = 0;
    return this.result;
  }

  reset(): void {
    this.requireBridge().reset();
  }

  private requireBridge(): FutenisBridge {
    if (!this.bridge) throw new Error("Engine no cargado: llamá load() primero");
    return this.bridge;
  }

  private writeInput(o: number, a: PlayerAction): void {
    const b = this.input;
    b[o] = a.moveX;
    b[o + 1] = a.moveZ;
    b[o + 2] = a.kick ? 1 : 0;
    b[o + 3] = a.aimX;
    b[o + 4] = a.aimZ;
    b[o + 5] = a.loft;
    b[o + 6] = a.power;
  }

  private readEvents(): void {
    const o = this.out;
    const count = Math.max(0, Math.min(MAX_EVENTS, o[EVENT_COUNT_OFFSET] | 0));
    for (let i = 0; i < count; i++) {
      const b = EVENTS_OFFSET + i * EVENT_STRIDE;
      const e = this.eventPool[i];
      e.kind = o[b];
      e.tick = o[b + 1];
      e.x = o[b + 2];
      e.y = o[b + 3];
      e.z = o[b + 4];
      e.side = o[b + 5];
    }
    this.result.eventCount = count;
  }
}
