import { addXp } from "./progress";
import { profileWriter } from "./store";

// Misiones locales: 4 diarias (reset a medianoche local) y 3 semanales (reset el lunes).
// Progreso desde los mismos eventos que el perfil (profile/recorder.ts).
export type Metric = "win" | "match" | "perfect" | "training" | "trainStreak";
export type Period = "day" | "week";

export interface MissionDef { id: string; title: string; metric: Metric; goal: number; xp: number; max?: boolean }
export interface MissionView { def: MissionDef; progress: number; claimed: boolean; claimable: boolean }

export const DAILY: readonly MissionDef[] = [
  { id: "d-win", title: "Ganá 1 partido", metric: "win", goal: 1, xp: 60 },
  { id: "d-perfect", title: "10 golpes perfectos", metric: "perfect", goal: 10, xp: 50 },
  { id: "d-train", title: "Completá 2 entrenamientos", metric: "training", goal: 2, xp: 50 },
  { id: "d-streak", title: "Racha de 5 en el lanzapelotas", metric: "trainStreak", goal: 5, xp: 40, max: true },
];
export const WEEKLY: readonly MissionDef[] = [
  { id: "w-win", title: "Ganá 5 partidos", metric: "win", goal: 5, xp: 250 },
  { id: "w-match", title: "Jugá 10 partidos", metric: "match", goal: 10, xp: 200 },
  { id: "w-perfect", title: "50 golpes perfectos", metric: "perfect", goal: 50, xp: 200 },
];

const KEY = "futenis.missions.v1";
const DAYS_PER_WEEK = 7;

interface Bucket { key: string; progress: Record<string, number>; claimed: string[] }
interface Saved { day: Bucket; week: Bucket }

const pad2 = (n: number): string => String(n).padStart(2, "0");
const dateKey = (d: Date): string => `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`;

export function periodKey(p: Period, now: Date): string {
  if (p === "day") return dateKey(now);
  const monday = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  monday.setDate(monday.getDate() - ((monday.getDay() + DAYS_PER_WEEK - 1) % DAYS_PER_WEEK));
  return dateKey(monday);
}

function bucket(raw: unknown, key: string): Bucket {
  const empty: Bucket = { key, progress: {}, claimed: [] };
  if (typeof raw !== "object" || raw === null) return empty;
  const r = raw as Record<string, unknown>;
  if (r.key !== key || typeof r.progress !== "object" || r.progress === null || !Array.isArray(r.claimed)) return empty;
  const progress: Record<string, number> = {};
  for (const [k, v] of Object.entries(r.progress as Record<string, unknown>)) {
    if (typeof v === "number" && Number.isFinite(v) && v >= 0) progress[k] = Math.floor(v);
  }
  return { key, progress, claimed: r.claimed.filter((c): c is string => typeof c === "string") };
}

function load(now: Date): Saved {
  let raw: Record<string, unknown> = {};
  try {
    const s = localStorage.getItem(KEY);
    const parsed: unknown = s === null ? null : JSON.parse(s);
    if (typeof parsed === "object" && parsed !== null) raw = parsed as Record<string, unknown>;
  } catch { /* corrupto o bloqueado: arranca vacío */ }
  return { day: bucket(raw.day, periodKey("day", now)), week: bucket(raw.week, periodKey("week", now)) };
}

export class Missions {
  private saved: Saved = load(new Date());
  private readonly subs = new Set<() => void>();

  onChange(cb: () => void): () => void { this.subs.add(cb); return () => { this.subs.delete(cb); }; }

  list(p: Period): MissionView[] {
    this.roll();
    const b = this.saved[p];
    return (p === "day" ? DAILY : WEEKLY).map((def) => {
      const progress = Math.min(def.goal, b.progress[def.id] ?? 0);
      const claimed = b.claimed.includes(def.id);
      return { def, progress, claimed, claimable: !claimed && progress >= def.goal };
    });
  }

  claimableCount(): number {
    return this.list("day").filter((m) => m.claimable).length + this.list("week").filter((m) => m.claimable).length;
  }

  /** Suma (o máximo, para rachas) al progreso de todas las misiones de esa métrica. */
  record(metric: Metric, value: number): void {
    this.roll();
    const next = { day: this.bump(this.saved.day, DAILY, metric, value), week: this.bump(this.saved.week, WEEKLY, metric, value) };
    this.commit(next);
  }

  claim(p: Period, id: string): void {
    const m = this.list(p).find((v) => v.def.id === id);
    if (!m || !m.claimable) return;
    const b = this.saved[p];
    this.commit({ ...this.saved, [p]: { ...b, claimed: [...b.claimed, id] } });
    profileWriter().update((s) => addXp(s, m.def.xp));
  }

  private bump(b: Bucket, defs: readonly MissionDef[], metric: Metric, value: number): Bucket {
    const progress = { ...b.progress };
    for (const d of defs) {
      if (d.metric !== metric) continue;
      const cur = progress[d.id] ?? 0;
      progress[d.id] = d.max ? Math.max(cur, value) : cur + value;
    }
    return { ...b, progress };
  }

  private roll(): void {
    const now = new Date();
    if (this.saved.day.key === periodKey("day", now) && this.saved.week.key === periodKey("week", now)) return;
    this.commit(load(now));
  }

  private commit(next: Saved): void {
    this.saved = next;
    try { localStorage.setItem(KEY, JSON.stringify(next)); } catch { /* sigue en memoria */ }
    this.subs.forEach((cb) => cb());
  }
}

let instance: Missions | null = null;
export function missions(): Missions {
  if (instance === null) instance = new Missions();
  return instance;
}
