// gzip for replay tapes (server TAPE messages, IndexedDB). Decompression is
// capped: a gzip bomb (from a compromised server or tampered storage) stops at
// MAX_TAPE_BYTES instead of eating the tab's memory.
export const MAX_TAPE_BYTES = 32 * 1024 * 1024;

export const canZip = (): boolean => typeof CompressionStream === "function" && typeof DecompressionStream === "function";

export async function gzip(data: Uint8Array): Promise<Uint8Array> {
  const out = new Response(new Blob([data.slice()]).stream().pipeThrough(new CompressionStream("gzip")));
  return new Uint8Array(await out.arrayBuffer());
}

/** null when the browser can't, the data is bad or the output passes max. */
export async function gunzip(gz: Uint8Array, max = MAX_TAPE_BYTES): Promise<Uint8Array | null> {
  if (!canZip()) return null;
  const reader = new Blob([gz.slice()]).stream().pipeThrough(new DecompressionStream("gzip")).getReader();
  const parts: Uint8Array[] = [];
  let size = 0;
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      size += value.byteLength;
      if (size > max) { await reader.cancel(); return null; }
      parts.push(value);
    }
  } catch {
    return null;
  }
  const out = new Uint8Array(size);
  let o = 0;
  for (const p of parts) { out.set(p, o); o += p.byteLength; }
  return out;
}
