export type GameMode = "1v1" | "2v2" | "playground";
export type Difficulty = "easy" | "normal" | "hard" | "pro";

export interface MatchOptions {
  readonly mode: GameMode;
  readonly difficulty: Difficulty;
  readonly pointsToWin: number;
}

export const POINTS_CHOICES: readonly number[] = [5, 7, 11, 21];
export const DEFAULT_POINTS = 11;
