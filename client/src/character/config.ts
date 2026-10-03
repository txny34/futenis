// Single source of body proportions for the procedural "toy athlete".
// Heights are world meters with feet at Y=0, standing idle. Values mirror the
// sim body-part table (sim/config_layout.go, contact-v2 §6) so they can later
// be replaced by getConfig() without touching geometry code.

export type StrikePart = "foot" | "knee" | "chest" | "shoulder" | "head" | "bicycle";

/** Visual strike tiers: the sim keeps 6 parts, the puppet animates 3. */
export type StrikeTier = "low" | "mid" | "high";

/** Single sim part -> animation tier mapping (chilena reads as a header for now). */
export const PART_TIER: Readonly<Record<StrikePart, StrikeTier>> = {
  foot: "low", knee: "low", chest: "mid", shoulder: "mid", head: "high", bicycle: "high",
};

export interface ZoneBand {
  readonly min: number;
  readonly ideal: number;
  readonly max: number;
}

/** Sim body-part bands (hRel, meters). Used for debug lines and pose targets. */
export const SIM_ZONES: Readonly<Record<StrikePart, ZoneBand>> = {
  foot: { min: 0.0, ideal: 0.2, max: 0.45 },
  knee: { min: 0.35, ideal: 0.6, max: 0.95 },
  chest: { min: 0.95, ideal: 1.2, max: 1.4 },
  shoulder: { min: 1.3, ideal: 1.5, max: 1.65 },
  head: { min: 1.6, ideal: 1.85, max: 2.1 },
  bicycle: { min: 1.7, ideal: 2.1, max: 2.5 },
};

/** Landmark heights the skeleton is built from (character-design §2). */
export const BODY = {
  height: 1.8,
  capsuleRadius: 0.35,
  toe: 0.06,
  ankle: 0.1,
  knee: 0.52,
  hip: 0.94,
  pelvis: 0.99,
  waist: 1.0,
  spine: 1.08,
  chest: 1.2,
  shoulder: 1.48,
  neck: 1.5,
  headBase: 1.53,
  headCenter: 1.66,
  headRadius: 0.14, // top = 1.80; ~6 heads tall (toy proportions)
  hipHalfWidth: 0.1,
  shoulderHalfWidth: 0.23,
  upperArm: 0.27,
  foreArm: 0.25,
  footForward: 0.13,
} as const;

export interface CharacterLook {
  shirt: number;
  shorts: number;
  skin: number;
  hair: number;
  boots: number;
}

export const DEFAULT_LOOK: Readonly<CharacterLook> = {
  shirt: 0xe5484d,
  shorts: 0xf2f2f2,
  skin: 0xe0ac7e,
  hair: 0x2b1d14,
  boots: 0x1f6feb,
};
