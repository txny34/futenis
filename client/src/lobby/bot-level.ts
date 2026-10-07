import type { Difficulty } from "../ui/types";

// Bot level is automatic (docs/design/lobby.md "Elegí un modo"): picked from the
// guest's ELO for the queue being played. No difficulty UI anywhere.
const EASY_BELOW = 900;
const NORMAL_BELOW = 1100;
const HARD_BELOW = 1300;

export function botLevelFor(elo: number): Difficulty {
  if (elo < EASY_BELOW) return "easy";
  if (elo < NORMAL_BELOW) return "normal";
  if (elo < HARD_BELOW) return "hard";
  return "pro";
}
