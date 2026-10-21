// client/src/replay/tape-format.ts
import { CFG_LEN, type HumanInput, INPUT_PER_PLAYER } from "../engine";
import { MAX_PLAYERS } from "../snapshot";

// Tape v2 (game/tape.go): start setup + config, then per step only the human
// slots whose input changed (equal steps merged). The server records online
// matches with it; local bot matches are recorded here. Frozen format.
const MAGIC = 0x32505446;
const VERSION = 2;
export const TAPE_HEADER_LEN = 16 + CFG_LEN * 8;
const MAX_RUN = 0xffff;
const IN = INPUT_PER_PLAYER; // 6 floats per slot
const INIT_BYTES = 256 * 1024;

export interface TapeHeader {
  mode: number; difficulty: number; initMask: number; pointsToWin: number; seat: number; firstServer: number;
}

/** Decoded records: inputs carried forward, MAX_PLAYERS * 6 floats per record. */
export interface DecodedTape {
  readonly header: TapeHeader;
  readonly cfg: Float64Array;
  readonly count: number;
  readonly ticks: Uint16Array;
  readonly masks: Uint8Array;
  readonly inputs: Float64Array;
  readonly totalTicks: number;
}

export class TapeWriter {
  private buf: Uint8Array;
  private view: DataView;
  private len = TAPE_HEADER_LEN;
  private readonly prev = new Float64Array(MAX_PLAYERS * IN);
  private prevMask: number;
  private last = -1;
  private records = 0;
  private ticks = 0;

  constructor(h: Readonly<TapeHeader>, cfg: Float64Array, private readonly maxTicks: number) {
    this.buf = new Uint8Array(INIT_BYTES);
    this.view = new DataView(this.buf.buffer);
    const v = this.view;
    v.setUint32(0, MAGIC, true);
    v.setUint8(4, VERSION); v.setUint8(5, h.mode); v.setUint8(6, h.difficulty); v.setUint8(7, h.initMask);
    v.setUint16(8, h.pointsToWin, true); v.setUint8(10, h.seat); v.setUint8(11, h.firstServer);
    this.buf.set(new Uint8Array(cfg.buffer, cfg.byteOffset, CFG_LEN * 8), 16);
    this.prevMask = h.initMask;
  }

  /** n ticks ran with inputs[slot] under mask. Allocation-free unless the buffer must grow. */
  frame(n: number, mask: number, inputs: readonly Readonly<HumanInput>[]): void {
    if (this.ticks > this.maxTicks) return;
    this.ticks += n;
    let changed = 0;
    for (let i = 0; i < MAX_PLAYERS; i++) {
      if ((mask & (1 << i)) !== 0 && differs(this.prev, i, inputs[i])) changed |= 1 << i;
    }
    if (this.last >= 0 && changed === 0 && mask === this.prevMask) {
      const t = this.view.getUint16(this.last, true);
      if (t + n <= MAX_RUN) { this.view.setUint16(this.last, t + n, true); return; }
    }
    this.append(n, mask, changed, inputs);
  }

  /** The finished tape, or null when it outgrew maxTicks. */
  bytes(): Uint8Array | null {
    if (this.ticks > this.maxTicks) return null;
    this.view.setUint32(12, this.records, true);
    return this.buf.slice(0, this.len);
  }

  private append(n: number, mask: number, changed: number, inputs: readonly Readonly<HumanInput>[]): void {
    this.ensure(4 + MAX_PLAYERS * IN * 8);
    const v = this.view;
    this.last = this.len;
    v.setUint16(this.len, n, true); v.setUint8(this.len + 2, mask); v.setUint8(this.len + 3, changed);
    this.len += 4;
    for (let i = 0; i < MAX_PLAYERS; i++) {
      if ((changed & (1 << i)) === 0) continue;
      const a = inputs[i];
      const vals = this.prev;
      const o = i * IN;
      vals[o] = a.moveX; vals[o + 1] = a.moveZ; vals[o + 2] = a.targetX; vals[o + 3] = a.targetZ; vals[o + 4] = a.buttons; vals[o + 5] = a.spin;
      for (let k = 0; k < IN; k++) { v.setFloat64(this.len, vals[o + k], true); this.len += 8; }
    }
    this.prevMask = mask;
    this.records++;
  }

  private ensure(extra: number): void {
    if (this.len + extra <= this.buf.length) return;
    const next = new Uint8Array(this.buf.length * 2);
    next.set(this.buf.subarray(0, this.len));
    this.buf = next;
    this.view = new DataView(next.buffer);
  }
}

function differs(prev: Float64Array, i: number, a: Readonly<HumanInput>): boolean {
  const o = i * IN;
  return prev[o] !== a.moveX || prev[o + 1] !== a.moveZ || prev[o + 2] !== a.targetX
    || prev[o + 3] !== a.targetZ || prev[o + 4] !== a.buttons || prev[o + 5] !== a.spin;
}

/** Parses a tape; throws on anything malformed (never trust stored/network bytes). */
export function decodeTape(b: Uint8Array): DecodedTape {
  const v = new DataView(b.buffer, b.byteOffset, b.byteLength);
  if (b.byteLength < TAPE_HEADER_LEN || v.getUint32(0, true) !== MAGIC || v.getUint8(4) !== VERSION) throw new Error("tape: bad header");
  const header: TapeHeader = {
    mode: v.getUint8(5), difficulty: v.getUint8(6), initMask: v.getUint8(7),
    pointsToWin: v.getUint16(8, true), seat: v.getUint8(10), firstServer: v.getUint8(11),
  };
  if (header.mode > 1 || header.seat >= MAX_PLAYERS || header.firstServer > 1) throw new Error("tape: bad setup");
  const count = v.getUint32(12, true);
  const cfg = new Float64Array(CFG_LEN);
  for (let i = 0; i < CFG_LEN; i++) cfg[i] = v.getFloat64(16 + i * 8, true);
  return decodeRecords(v, header, cfg, count);
}

function decodeRecords(v: DataView, header: TapeHeader, cfg: Float64Array, count: number): DecodedTape {
  const ticks = new Uint16Array(count);
  const masks = new Uint8Array(count);
  const inputs = new Float64Array(count * MAX_PLAYERS * IN);
  const cur = new Float64Array(MAX_PLAYERS * IN);
  let off = TAPE_HEADER_LEN;
  let total = 0;
  for (let r = 0; r < count; r++) {
    if (off + 4 > v.byteLength) throw new Error("tape: truncated");
    ticks[r] = v.getUint16(off, true); masks[r] = v.getUint8(off + 2);
    const changed = v.getUint8(off + 3);
    off += 4;
    for (let i = 0; i < MAX_PLAYERS; i++) {
      if ((changed & (1 << i)) === 0) continue;
      if (off + IN * 8 > v.byteLength) throw new Error("tape: truncated");
      for (let k = 0; k < IN; k++) { cur[i * IN + k] = v.getFloat64(off, true); off += 8; }
    }
    inputs.set(cur, r * MAX_PLAYERS * IN);
    total += ticks[r];
  }
  return { header, cfg, count, ticks, masks, inputs, totalTicks: total };
}

/** Stamps the viewer's seat (byte 10) before saving a tape received from the server. */
export function setTapeSeat(b: Uint8Array, seat: number): void { b[10] = seat; }

/** Reads record r's slot input into out. */
export function tapeInput(t: DecodedTape, r: number, slot: number, out: HumanInput): void {
  const o = (r * MAX_PLAYERS + slot) * IN;
  const x = t.inputs;
  out.moveX = x[o]; out.moveZ = x[o + 1]; out.targetX = x[o + 2]; out.targetZ = x[o + 3]; out.buttons = x[o + 4]; out.spin = x[o + 5];
}
