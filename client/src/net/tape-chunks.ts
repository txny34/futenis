import { HEADER, Msg, PROTO_VERSION } from "./protocol";

// TAPE (cmd/server/internal/wire/tape.go): the finished match's gzipped replay
// tape in ordered chunks; header bytes 2..3 = index, 4..5 = count (u16 LE).
const MAX_CHUNKS = 512;
const MAX_BYTES = 16 * 1024 * 1024;

export class TapeAssembler {
  private parts: Uint8Array[] = [];
  private count = 0;
  private bytes = 0;

  /** Feeds one TAPE message; returns the whole gzip payload after the last chunk. */
  push(msg: Uint8Array): Uint8Array | null {
    if (msg.length < HEADER || msg[0] !== Msg.Tape || msg[1] !== PROTO_VERSION) return null;
    const v = new DataView(msg.buffer, msg.byteOffset, msg.byteLength);
    const index = v.getUint16(2, true);
    const count = v.getUint16(4, true);
    if (count === 0 || count > MAX_CHUNKS || index >= count) { this.reset(); return null; }
    if (index === 0) this.reset();
    if (index !== this.parts.length || (this.count !== 0 && count !== this.count)) { this.reset(); return null; }
    this.count = count;
    this.bytes += msg.length - HEADER;
    if (this.bytes > MAX_BYTES) { this.reset(); return null; }
    this.parts.push(msg.slice(HEADER));
    if (this.parts.length < count) return null;
    const out = new Uint8Array(this.bytes);
    let o = 0;
    for (const p of this.parts) { out.set(p, o); o += p.length; }
    this.reset();
    return out;
  }

  private reset(): void { this.parts = []; this.count = 0; this.bytes = 0; }
}

/** gunzip; null when the browser can't or the data is bad. */
export async function gunzip(gz: Uint8Array): Promise<Uint8Array | null> {
  if (typeof DecompressionStream !== "function") return null;
  try {
    const stream = new Blob([gz.slice()]).stream().pipeThrough(new DecompressionStream("gzip"));
    return new Uint8Array(await new Response(stream).arrayBuffer());
  } catch {
    return null;
  }
}

/** Assembles + gunzips TAPE chunks and fires onDone once per completed tape, with the seat at receive time. */
export class TapeReceiver {
  private readonly tapes = new TapeAssembler();

  constructor(private readonly onDone: (tape: Uint8Array, seat: number) => void) {}

  push(b: Uint8Array, seat: number): void {
    const gz = this.tapes.push(b);
    if (!gz) return;
    void gunzip(gz).then((raw) => { if (raw) this.onDone(raw, seat); });
  }
}
