import type { Looks } from "../net/style-wire";

// Replays of the last matches (IndexedDB futenis-replays, this browser only):
// gzipped tape v2 + what Historial shows. Keyed by the history record's `at`.
// Storage can be blocked: every call resolves (false / null / empty).
const DB = "futenis-replays";
const STORE = "replays";
const VERSION = 1;
export const REPLAYS_MAX = 10;

export interface ReplayMeta {
  readonly id: number; // MatchRecord.at
  readonly queue: "1v1" | "2v2";
  readonly online: boolean;
  readonly ranked: boolean;
  readonly won: boolean;
  readonly games0: number;
  readonly games1: number;
  readonly looks: Looks | null; // online: every seat's look in view order
}

interface Row { readonly id: number; readonly meta: ReplayMeta; readonly gz: boolean; readonly data: Uint8Array }

function open(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(DB, VERSION);
    req.onupgradeneeded = () => { req.result.createObjectStore(STORE, { keyPath: "id" }); };
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error ?? new Error("indexedDB open failed"));
  });
}

function done<T>(req: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error ?? new Error("indexedDB request failed"));
  });
}

async function withStore<T>(mode: IDBTransactionMode, fn: (s: IDBObjectStore) => Promise<T>): Promise<T> {
  const db = await open();
  try {
    return await fn(db.transaction(STORE, mode).objectStore(STORE));
  } finally {
    db.close();
  }
}

async function pipe(data: Uint8Array, t: CompressionStream | DecompressionStream): Promise<Uint8Array> {
  const out = new Response(new Blob([data.slice()]).stream().pipeThrough(t));
  return new Uint8Array(await out.arrayBuffer());
}

const canZip = (): boolean => typeof CompressionStream === "function" && typeof DecompressionStream === "function";

function isRow(v: unknown): v is Row {
  const r = v as Partial<Row> | null;
  const m = r?.meta as Partial<ReplayMeta> | undefined;
  return !!r && typeof r.id === "number" && r.data instanceof Uint8Array && typeof r.gz === "boolean"
    && !!m && m.id === r.id && (m.queue === "1v1" || m.queue === "2v2") && typeof m.won === "boolean"
    && typeof m.online === "boolean" && typeof m.games0 === "number" && typeof m.games1 === "number";
}

/** Saves a replay and keeps only the newest REPLAYS_MAX. */
export async function saveReplay(meta: ReplayMeta, tape: Uint8Array): Promise<boolean> {
  try {
    const gz = canZip();
    const data = gz ? await pipe(tape, new CompressionStream("gzip")) : tape.slice();
    const row: Row = { id: meta.id, meta, gz, data };
    await withStore("readwrite", async (s) => {
      await done(s.put(row));
      const keys = (await done(s.getAllKeys())).filter((k): k is number => typeof k === "number").sort((a, b) => b - a);
      for (const old of keys.slice(REPLAYS_MAX)) await done(s.delete(old));
    });
    return true;
  } catch {
    return false;
  }
}

export async function loadReplay(id: number): Promise<{ meta: ReplayMeta; tape: Uint8Array } | null> {
  try {
    const row: unknown = await withStore("readonly", (s) => done(s.get(id)));
    if (!isRow(row)) return null;
    const tape = row.gz ? await pipe(row.data, new DecompressionStream("gzip")) : row.data;
    return { meta: row.meta, tape };
  } catch {
    return null;
  }
}

export async function replayIds(): Promise<Set<number>> {
  try {
    const keys = await withStore("readonly", (s) => done(s.getAllKeys()));
    return new Set(keys.filter((k): k is number => typeof k === "number"));
  } catch {
    return new Set();
  }
}
