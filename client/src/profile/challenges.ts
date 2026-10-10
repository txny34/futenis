import { PART_NAMES, EventKind, Quality, SimEvent } from "../snapshot";
import { t } from "../i18n/t";

// Desafíos de Entrenamiento (backlog "Aprender y mejorar"): per-session goals
// with bronze/silver/gold medals; the best medal per challenge persists locally.
// Pure reader of the event stream, like the profile recorder.
export const MEDALS = [t("Bronce", "Bronze"), t("Plata", "Silver"), t("Oro", "Gold")] as const;
export type MedalIdx = 0 | 1 | 2;

type Metric = "streak" | "perfect" | "headPerfect" | "chilena";

export interface ChallengeDef { readonly id: string; readonly title: string; readonly metric: Metric; readonly tiers: readonly [number, number, number] }

export const CHALLENGES: readonly ChallengeDef[] = [
  { id: "c-streak", title: t("Racha en el lanzapelotas", "Streak on the ball machine"), metric: "streak", tiers: [5, 10, 20] },
  { id: "c-perfect", title: t("Golpes perfectos en una sesión", "Perfect strikes in one session"), metric: "perfect", tiers: [5, 15, 30] },
  { id: "c-head", title: t("Perfects de cabeza", "Header perfects"), metric: "headPerfect", tiers: [2, 5, 10] },
  { id: "c-chilena", title: t("Chilenas", "Bicycle kicks"), metric: "chilena", tiers: [1, 3, 5] },
];

const KEY = "futenis.challenges.v1";
const HUMAN_SLOT = 0;
const NO_MEDAL = -1;
const NONE_EARNED: readonly never[] = [];

/** challenge id -> best medal index (absent = none). */
type Best = Record<string, number>;

function load(): Best {
  try {
    const raw: unknown = JSON.parse(localStorage.getItem(KEY) ?? "{}");
    if (typeof raw !== "object" || raw === null) return {};
    const out: Best = {};
    for (const c of CHALLENGES) {
      const v = (raw as Record<string, unknown>)[c.id];
      if (v === 0 || v === 1 || v === 2) out[c.id] = v;
    }
    return out;
  } catch { return {}; }
}

function save(b: Best): void {
  try { localStorage.setItem(KEY, JSON.stringify(b)); } catch { /* storage blocked: medals last this visit */ }
}

/** Best medal per challenge id (absent = none), read once. */
export function bestMedals(): Readonly<Record<string, number>> { return load(); }

export interface MedalEarned { readonly def: ChallengeDef; readonly medal: MedalIdx }

/** Counts one training session; onEvent reports a medal the first time a tier above the saved best is reached. */
export class ChallengeSession {
  private readonly count: Record<Metric, number> = { streak: 0, perfect: 0, headPerfect: 0, chilena: 0 };
  private run = 0; // live streak (streak metric keeps the session max)
  private best: Best = {};
  private active = false;

  begin(): void {
    this.active = true;
    this.best = { ...load() };
    this.run = 0;
    for (const k of Object.keys(this.count) as Metric[]) this.count[k] = 0;
  }

  end(): void { this.active = false; }

  /** Medals newly reached by this event (usually none; shared empty array then). */
  onEvent(e: SimEvent): readonly MedalEarned[] {
    if (!this.active || !this.track(e)) return NONE_EARNED;
    const earned: MedalEarned[] = [];
    const stored = load(); // another tab may have saved meanwhile: keep the max
    for (const def of CHALLENGES) {
      const got = this.medalFor(def);
      const prev = Math.max(this.best[def.id] ?? NO_MEDAL, stored[def.id] ?? NO_MEDAL);
      if (got > prev) earned.push({ def, medal: got as MedalIdx });
      this.best[def.id] = Math.max(prev, got);
    }
    if (earned.length > 0) save(this.best);
    return earned;
  }

  /** Updates the counters; true when any changed. */
  private track(e: SimEvent): boolean {
    if (e.kind === EventKind.ReturnOK) {
      this.run++;
      this.count.streak = Math.max(this.count.streak, this.run);
      return true;
    }
    if (e.kind === EventKind.ReturnFail) { this.run = 0; return false; }
    if (e.kind !== EventKind.Kick || e.player !== HUMAN_SLOT) return false;
    const part = PART_NAMES[e.part];
    if (part === "bicycle") this.count.chilena++;
    if (e.quality === Quality.Perfect) {
      this.count.perfect++;
      if (part === "head") this.count.headPerfect++;
    }
    return true;
  }

  private medalFor(def: ChallengeDef): number {
    const v = this.count[def.metric];
    let m = NO_MEDAL;
    for (let i = 0; i < def.tiers.length; i++) if (v >= def.tiers[i]) m = i;
    return m;
  }
}
