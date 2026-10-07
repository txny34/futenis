import { ProfileStats, Queue, xpToNextFor } from "./store";

// Reglas puras de progresión local (docs/design/lobby.md "Perfil local y misiones").
export const XP_WIN = 100;
export const XP_LOSS = 40;
export const XP_PERFECT = 5;
export const XP_TRAINING = 30;
export const ELO_K = 32;
const ELO_SCALE = 400;

export type BotDifficulty = "easy" | "normal" | "hard" | "pro";
export const BOT_RATING: Readonly<Record<BotDifficulty, number>> = { easy: 800, normal: 1000, hard: 1200, pro: 1400 };

/** Suma XP y sube de nivel tantas veces como alcance (xpToNext = 100 + 40·nivel). */
export function addXp(s: ProfileStats, amount: number): ProfileStats {
  let level = s.level;
  let xp = s.xp + Math.max(0, Math.floor(amount));
  let need = xpToNextFor(level);
  while (xp >= need) {
    xp -= need;
    level += 1;
    need = xpToNextFor(level);
  }
  return { ...s, level, xp, xpToNext: need };
}

/** Elo clásico contra un rating fijo de bot. */
export function eloAfter(rating: number, opponent: number, won: boolean): number {
  const expected = 1 / (1 + Math.pow(10, (opponent - rating) / ELO_SCALE));
  return Math.round(rating + ELO_K * ((won ? 1 : 0) - expected));
}

export function applyMatch(s: ProfileStats, queue: Queue, bot: BotDifficulty, won: boolean): ProfileStats {
  const streak = won ? s.streak + 1 : 0;
  const next: ProfileStats = {
    ...s,
    elo: { ...s.elo, [queue]: eloAfter(s.elo[queue], BOT_RATING[bot], won) },
    wins: s.wins + (won ? 1 : 0),
    losses: s.losses + (won ? 0 : 1),
    streak,
    bestStreak: Math.max(s.bestStreak, streak),
  };
  return addXp(next, won ? XP_WIN : XP_LOSS);
}

export function applyPerfect(s: ProfileStats): ProfileStats {
  return addXp({ ...s, perfects: s.perfects + 1 }, XP_PERFECT);
}

export function applyTraining(s: ProfileStats, bestStreak: number): ProfileStats {
  const next = { ...s, trainings: s.trainings + 1, bestTrainingStreak: Math.max(s.bestTrainingStreak, bestStreak) };
  return addXp(next, XP_TRAINING);
}
