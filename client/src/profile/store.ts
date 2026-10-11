// Perfil local (invitado). API CONGELADA: Agent Z (zone2) depende de get/onChange.
export type Queue = "1v1" | "2v2";

export interface ProfileStats {
  name: string;
  level: number; xp: number; xpToNext: number;
  elo: Record<Queue, number>;
  wins: number; losses: number; streak: number; bestStreak: number;
  perfects: number; trainings: number; bestTrainingStreak: number;
}

export interface ProfileStore {
  get(): Readonly<ProfileStats>;
  onChange(cb: (s: Readonly<ProfileStats>) => void): () => void;
}

/** Superficie de escritura (solo profile/record.ts y misiones). */
export interface ProfileWriter extends ProfileStore {
  update(fn: (s: ProfileStats) => ProfileStats): void;
  /** Re-reads localStorage (cloud sync wrote a merged copy). */
  reload(): void;
}

const KEY = "futenis.profile.v1";
export const START_ELO = 1000;

export function xpToNextFor(level: number): number {
  return 100 + 40 * level;
}

function defaults(): ProfileStats {
  return {
    name: "Invitado", level: 1, xp: 0, xpToNext: xpToNextFor(1),
    elo: { "1v1": START_ELO, "2v2": START_ELO },
    wins: 0, losses: 0, streak: 0, bestStreak: 0,
    perfects: 0, trainings: 0, bestTrainingStreak: 0,
  };
}

function num(v: unknown, fb: number, min: number, max: number): number {
  return typeof v === "number" && Number.isFinite(v) ? Math.min(max, Math.max(min, Math.floor(v))) : fb;
}

function validate(raw: unknown): ProfileStats {
  const d = defaults();
  if (typeof raw !== "object" || raw === null) return d;
  const r = raw as Record<string, unknown>;
  const e = (typeof r.elo === "object" && r.elo !== null ? r.elo : {}) as Record<string, unknown>;
  const BIG = 1e9;
  const level = num(r.level, d.level, 1, 999);
  const name = typeof r.name === "string" && r.name.trim() !== "" ? r.name.slice(0, 20) : d.name;
  return {
    name, level, xp: num(r.xp, 0, 0, xpToNextFor(level) - 1), xpToNext: xpToNextFor(level),
    elo: { "1v1": num(e["1v1"], START_ELO, 0, 5000), "2v2": num(e["2v2"], START_ELO, 0, 5000) },
    wins: num(r.wins, 0, 0, BIG), losses: num(r.losses, 0, 0, BIG),
    streak: num(r.streak, 0, 0, BIG), bestStreak: num(r.bestStreak, 0, 0, BIG),
    perfects: num(r.perfects, 0, 0, BIG), trainings: num(r.trainings, 0, 0, BIG),
    bestTrainingStreak: num(r.bestTrainingStreak, 0, 0, BIG),
  };
}

function load(): ProfileStats {
  try {
    const s = localStorage.getItem(KEY);
    return s === null ? defaults() : validate(JSON.parse(s));
  } catch {
    return defaults();
  }
}

function save(s: ProfileStats): void {
  try { localStorage.setItem(KEY, JSON.stringify(s)); } catch { /* storage bloqueado: sigue en memoria */ }
}

function create(): ProfileWriter {
  let state: Readonly<ProfileStats> = load();
  const subs = new Set<(s: Readonly<ProfileStats>) => void>();
  return {
    get: () => state,
    onChange(cb) { subs.add(cb); return () => { subs.delete(cb); }; },
    update(fn) {
      state = validate(fn({ ...state, elo: { ...state.elo } }));
      save(state);
      subs.forEach((cb) => cb(state));
    },
    reload() {
      state = load();
      subs.forEach((cb) => cb(state));
    },
  };
}

let instance: ProfileWriter | null = null;

export function profileWriter(): ProfileWriter {
  if (instance === null) instance = create();
  return instance;
}

export function profileStore(): ProfileStore {
  return profileWriter();
}
