// Merging two copies of the progress (this browser and the account's cloud row).
// Pure and loose: every part is re-validated by its own module when loaded, so
// here we only pick values. Max semantics: playing on two devices never loses
// the best of each, and nothing is counted twice (claimed missions are a union).
export const PARTS = {
  profile: "futenis.profile.v1",
  missions: "futenis.missions.v1",
  history: "futenis.history.v1",
  challenges: "futenis.challenges.v1",
} as const;

export type Part = keyof typeof PARTS;
export type Snapshot = Partial<Record<Part, unknown>>;

const HISTORY_MAX = 20;
const COUNTERS = ["wins", "losses", "streak", "bestStreak", "perfects", "trainings", "bestTrainingStreak"] as const;
const DEFAULT_NAME = "Invitado";

type Obj = Record<string, unknown>;
const obj = (v: unknown): Obj | null => (typeof v === "object" && v !== null && !Array.isArray(v) ? (v as Obj) : null);
const n = (v: unknown): number => (typeof v === "number" && Number.isFinite(v) ? v : 0);

function mergeProfile(a: Obj, b: Obj): Obj {
  const aAhead = n(a.level) > n(b.level) || (n(a.level) === n(b.level) && n(a.xp) >= n(b.xp));
  const lead = aAhead ? a : b;
  const out: Obj = { ...b, ...a, level: lead.level, xp: lead.xp };
  for (const k of COUNTERS) out[k] = Math.max(n(a[k]), n(b[k]));
  const named = (v: unknown): boolean => typeof v === "string" && v.trim() !== "" && v !== DEFAULT_NAME;
  out.name = named(a.name) ? a.name : b.name ?? a.name;
  return out;
}

function mergeBucket(a: unknown, b: unknown): unknown {
  const x = obj(a);
  const y = obj(b);
  if (!x || !y) return x ?? y;
  if (x.key !== y.key) return String(x.key) > String(y.key) ? x : y; // newer day/week wins
  const px = obj(x.progress) ?? {};
  const py = obj(y.progress) ?? {};
  const progress: Record<string, number> = {};
  for (const id of new Set([...Object.keys(px), ...Object.keys(py)])) progress[id] = Math.max(n(px[id]), n(py[id]));
  const claimed = [...new Set([...(Array.isArray(x.claimed) ? x.claimed : []), ...(Array.isArray(y.claimed) ? y.claimed : [])])];
  return { key: x.key, progress, claimed };
}

function mergeHistory(a: unknown, b: unknown): unknown[] {
  const all = [...(Array.isArray(a) ? a : []), ...(Array.isArray(b) ? b : [])];
  const seen = new Set<string>();
  const out: unknown[] = [];
  for (const r of all) {
    const o = obj(r);
    if (!o) continue;
    const id = `${n(o.at)}|${String(o.queue)}`;
    if (seen.has(id)) continue;
    seen.add(id);
    out.push(o);
  }
  return out.sort((p, q) => n(obj(q)?.at) - n(obj(p)?.at)).slice(0, HISTORY_MAX);
}

function mergeMax(a: Obj, b: Obj): Obj {
  const out: Obj = {};
  for (const k of new Set([...Object.keys(a), ...Object.keys(b)])) out[k] = Math.max(n(a[k]), n(b[k]));
  return out;
}

/** local ⊕ cloud. Either side may miss parts (fresh browser, first sync). */
export function mergeSnapshots(local: Snapshot, cloud: Snapshot): Snapshot {
  const out: Snapshot = {};
  const lp = obj(local.profile);
  const cp = obj(cloud.profile);
  out.profile = lp && cp ? mergeProfile(lp, cp) : lp ?? cp ?? undefined;
  const lm = obj(local.missions);
  const cm = obj(cloud.missions);
  out.missions = lm && cm ? { day: mergeBucket(lm.day, cm.day), week: mergeBucket(lm.week, cm.week) } : lm ?? cm ?? undefined;
  out.history = mergeHistory(local.history, cloud.history);
  const lc = obj(local.challenges);
  const cc = obj(cloud.challenges);
  out.challenges = lc && cc ? mergeMax(lc, cc) : lc ?? cc ?? undefined;
  return out;
}

/** The cloud row's data column, defensively (anything not an object = empty). */
export function asSnapshot(v: unknown): Snapshot {
  const o = obj(v);
  if (!o) return {};
  const out: Snapshot = {};
  for (const p of Object.keys(PARTS) as Part[]) if (p in o) out[p] = o[p];
  return out;
}
