import type { TrainingSettings } from "./training-settings";

export type GameMode = "1v1" | "2v2" | "playground";
/** Bot level: never picked by the player, derived from ELO (lobby/bot-level.ts). */
export type Difficulty = "easy" | "normal" | "hard" | "pro";

export interface MatchOptions {
  readonly mode: GameMode;
  readonly difficulty: Difficulty;
  /** Clasificación: ELO moves and XP x1.5. Casual: ELO unchanged. */
  readonly ranked: boolean;
  readonly pointsToWin: number;
  /** Entrenamiento ball-machine settings (playground only). */
  readonly training?: TrainingSettings;
  /** Guided tutorial over the ball machine (playground only). */
  readonly tutorial?: boolean;
}

export const DEFAULT_POINTS = 11;
