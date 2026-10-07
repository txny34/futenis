import type { Difficulty } from "../ui/types";

// Bot difficulty picked in the lobby menu (Esc) or a pad's setup screen; used by
// JUGAR and as the pads' default. Persisted in localStorage (try/catch: storage may be blocked).
const DIFFICULTY_KEY = "futenis.difficulty";
const DEFAULT_DIFFICULTY: Difficulty = "normal";
const VALUES: readonly Difficulty[] = ["easy", "normal", "hard", "pro"];

const isDifficulty = (v: string | null): v is Difficulty => VALUES.some((d) => d === v);

function load(): Difficulty {
  try {
    const v = window.localStorage.getItem(DIFFICULTY_KEY);
    return isDifficulty(v) ? v : DEFAULT_DIFFICULTY;
  } catch {
    return DEFAULT_DIFFICULTY;
  }
}

let current: Difficulty = load();

export function getDifficulty(): Difficulty { return current; }

export function setDifficulty(d: Difficulty): void {
  current = d;
  try { window.localStorage.setItem(DIFFICULTY_KEY, d); } catch { /* storage blocked: keep the in-memory value */ }
}
