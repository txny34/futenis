// Facial expressions as data (the Schedule I approach): an emotion is a mouth
// texture cell plus how open the eyelids are, the brow angle/height and the
// pupil size. The face blends lids/brows/pupils toward the preset and swaps the
// mouth cell at once. Visual only: nothing here reaches the sim.

/** Cells of the mouth atlas (mouth-atlas.ts draws them in this order). */
export const Mouth = {
  Neutral: 0, Smile: 1, Grin: 2, Frown: 3, Oh: 4, Gritted: 5, Smirk: 6, Shout: 7,
} as const;
export type MouthCell = (typeof Mouth)[keyof typeof Mouth];
export const MOUTH_CELLS = 8;

export type Emotion =
  | "neutral" | "focus" | "effort" | "happy" | "cheer" | "smug" | "sad" | "angry" | "surprised";

export interface Expression {
  readonly mouth: MouthCell;
  /** Upper lid: 1 = wide open, 0 = closed. */
  readonly lid: number;
  /** Brow roll in radians, + = inner ends up (worried), - = inner ends down (angry). */
  readonly browTilt: number;
  /** Brow lift in meters over rest, + = raised. */
  readonly browLift: number;
  /** Pupil scale (1 = rest). */
  readonly pupil: number;
}

export const EXPRESSIONS: Readonly<Record<Emotion, Expression>> = {
  neutral: { mouth: Mouth.Neutral, lid: 0.82, browTilt: 0, browLift: 0, pupil: 1 },
  focus: { mouth: Mouth.Neutral, lid: 0.55, browTilt: -0.22, browLift: -0.008, pupil: 0.85 },
  effort: { mouth: Mouth.Gritted, lid: 0.45, browTilt: -0.35, browLift: -0.01, pupil: 0.8 },
  happy: { mouth: Mouth.Smile, lid: 0.7, browTilt: 0.08, browLift: 0.008, pupil: 1.05 },
  cheer: { mouth: Mouth.Shout, lid: 1, browTilt: 0.12, browLift: 0.018, pupil: 1.1 },
  smug: { mouth: Mouth.Smirk, lid: 0.5, browTilt: -0.1, browLift: 0.004, pupil: 0.95 },
  sad: { mouth: Mouth.Frown, lid: 0.6, browTilt: 0.35, browLift: 0.006, pupil: 1.1 },
  angry: { mouth: Mouth.Gritted, lid: 0.62, browTilt: -0.45, browLift: -0.012, pupil: 0.75 },
  surprised: { mouth: Mouth.Oh, lid: 1, browTilt: 0.1, browLift: 0.022, pupil: 0.7 },
};
