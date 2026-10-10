import { CFG_LEN, type HumanInput } from "../engine";

// Replay tape (backlog "Repeticiones"): the sim is deterministic, so a match
// is fully described by its start config + every step call's tick count and
// the human's input. One record = [n, moveX, moveZ, targetX, targetZ, buttons, spin].
// The start config matters: configure() alternates the first server
// (game/replay_test.go). Live config edits mid-match (?dev=1 panel) are not
// recorded, so a tuned match replays with its start config.
const REC = 7;
const PREALLOC_RECORDS = 144 * 60 * 10; // 10 min of frames at 144 fps, allocated at the first match start
const MAX_RECORDS = 144 * 60 * 30; // longer matches stop recording (truncated: no replay)

export class Tape {
  private buf = new Float64Array(0);
  private len = 0;
  private full = false;
  /** Config snapshot taken when recording started (engine.getConfig()). */
  readonly startCfg = new Float64Array(CFG_LEN);

  get length(): number { return this.len; }
  /** True when the match outgrew MAX_RECORDS. */
  get truncated(): boolean { return this.full; }

  /** New recording from cfg (match start / R). Preallocates outside the frame loop. */
  clear(cfg: Float64Array | null = null): void {
    this.len = 0;
    this.full = false;
    if (cfg) this.startCfg.set(cfg);
    if (this.buf.length === 0) this.buf = new Float64Array(PREALLOC_RECORDS * REC);
  }

  /** Grows by doubling only past PREALLOC_RECORDS (rare: matches over 10 min). */
  push(n: number, a: Readonly<HumanInput>): void {
    if (this.len >= MAX_RECORDS) { this.full = true; return; }
    if ((this.len + 1) * REC > this.buf.length) {
      const next = new Float64Array(Math.min(Math.max(this.buf.length * 2, PREALLOC_RECORDS * REC), MAX_RECORDS * REC));
      next.set(this.buf);
      this.buf = next;
    }
    const o = this.len * REC;
    const b = this.buf;
    b[o] = n; b[o + 1] = a.moveX; b[o + 2] = a.moveZ; b[o + 3] = a.targetX;
    b[o + 4] = a.targetZ; b[o + 5] = a.buttons; b[o + 6] = a.spin;
    this.len++;
  }

  /** Tick count of record i. */
  ticks(i: number): number { return this.buf[i * REC]; }

  /** Writes record i's input into out and returns its tick count. */
  read(i: number, out: HumanInput): number {
    const o = i * REC;
    const b = this.buf;
    out.moveX = b[o + 1]; out.moveZ = b[o + 2]; out.targetX = b[o + 3];
    out.targetZ = b[o + 4]; out.buttons = b[o + 5]; out.spin = b[o + 6];
    return b[o];
  }
}
