import type { Queue } from "./store";

// Historial local de partidos (localStorage futenis.history.v1): los últimos 20,
// el más nuevo primero. Lo escribe recorder.matchOver; lo lee el modal de JUGAR.
const KEY = "futenis.history.v1";
export const HISTORY_MAX = 20;

export interface MatchRecord {
  readonly at: number; // epoch ms
  readonly queue: Queue;
  readonly ranked: boolean;
  readonly won: boolean;
  readonly games0: number;
  readonly games1: number;
  readonly eloDelta: number;
}

const int = (v: unknown): v is number => typeof v === "number" && Number.isInteger(v);

function valid(raw: unknown): raw is MatchRecord {
  if (typeof raw !== "object" || raw === null) return false;
  const r = raw as Record<string, unknown>;
  return int(r.at) && (r.queue === "1v1" || r.queue === "2v2") && typeof r.ranked === "boolean"
    && typeof r.won === "boolean" && int(r.games0) && int(r.games1) && int(r.eloDelta);
}

export function loadHistory(): readonly MatchRecord[] {
  try {
    const s = localStorage.getItem(KEY);
    const raw: unknown = s === null ? [] : JSON.parse(s);
    return Array.isArray(raw) ? raw.filter(valid).slice(0, HISTORY_MAX) : [];
  } catch {
    return [];
  }
}

export function pushHistory(r: MatchRecord): void {
  const next = [r, ...loadHistory()].slice(0, HISTORY_MAX);
  try { localStorage.setItem(KEY, JSON.stringify(next)); } catch { /* storage bloqueado: sin historial */ }
}
