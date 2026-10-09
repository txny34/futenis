/** Own-buffer byte array: what `new Uint8Array(n)` returns, and what WebSocket.send accepts
 * (unlike the generic Uint8Array<ArrayBufferLike> type, which also covers SharedArrayBuffer views). */
export type OwnBytes = Uint8Array<ArrayBuffer>;

/** Fixed-capacity FIFO of byte messages released after a delay. Preallocated: no per-message
 * allocations on push or drain (slots, lengths, due-timestamps and DataViews are built once). */
export class LagQueue {
  private readonly slots: OwnBytes[];
  private readonly views: DataView[];
  private readonly lens: Int32Array;
  private readonly due: Float64Array;
  private head = 0;
  private size = 0;

  constructor(capacity: number, slotBytes: number, private readonly delayMs: number) {
    this.slots = Array.from({ length: capacity }, () => new Uint8Array(slotBytes));
    this.views = this.slots.map((s) => new DataView(s.buffer));
    this.lens = new Int32Array(capacity);
    this.due = new Float64Array(capacity);
  }

  /** Copies msg in; when full, drops the oldest (keeps the freshest state). */
  push(msg: OwnBytes, now: number): void {
    if (this.size === this.slots.length) this.dropOldest();
    const i = (this.head + this.size) % this.slots.length;
    const n = Math.min(msg.length, this.slots[i].length);
    this.slots[i].set(msg.subarray(0, n));
    this.lens[i] = n;
    this.due[i] = now + this.delayMs;
    this.size++;
  }

  /** Calls fn for every message due at now, oldest first. fn must copy if it keeps the bytes;
   * msg and view are the slot's preallocated buffers, reused on the next push. */
  drain(now: number, fn: (msg: OwnBytes, len: number, view: DataView) => void): void {
    while (this.size > 0 && this.due[this.head] <= now) {
      const i = this.head;
      fn(this.slots[i], this.lens[i], this.views[i]);
      this.dropOldest();
    }
  }

  private dropOldest(): void {
    this.head = (this.head + 1) % this.slots.length;
    this.size--;
  }
}
