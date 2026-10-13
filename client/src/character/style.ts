import { THEME } from "../env/theme";

// Customizable look beyond team colors: hair and headwear shapes, skin/hair/hat
// palette indices, head size and build. Visual only: every option stays
// inside the sim's 1.80 m capsule (headSize widens, never heightens; build
// stays under the 0.70 m shoulder budget).

export const HAIR_STYLES = ["short", "buzz", "mohawk", "afro", "bald", "ponytail"] as const;
export type HairStyle = (typeof HAIR_STYLES)[number];
export const HEADWEAR = ["none", "cap", "headband", "capBack", "beanie"] as const;
export type Headwear = (typeof HEADWEAR)[number];
export const EYEWEAR = ["none", "sport", "round", "shades"] as const;
export type Eyewear = (typeof EYEWEAR)[number];
export const FACIAL_HAIR = ["none", "stubble", "mustache", "beard"] as const;
export type FacialHair = (typeof FACIAL_HAIR)[number];

export interface CharacterStyle {
  hair: HairStyle;
  headwear: Headwear;
  /** Indices into THEME.fx.skinTones / hairColors / hatColors. */
  skin: number;
  hairColor: number;
  hatColor: number;
  /** Head size: 0.92..1.08 (1 = default). */
  headSize: number;
  /** Shoulder/torso width: 0.92..1.06 (1 = default). */
  build: number;
  /** Shirt number 0..99 (printed on the back). */
  number: number;
  eyewear: Eyewear;
  facialHair: FacialHair;
  /** Wristbands on both wrists. */
  wristbands: boolean;
  /** Indices into THEME.fx.eyewearColors / gearColors (wristbands) / bootColors (sneakers). */
  eyewearColor: number;
  wristColor: number;
  bootsColor: number;
}

export const HEAD_SIZE_RANGE = [0.92, 1.08] as const;
export const BUILD_RANGE = [0.92, 1.06] as const;
export const MAX_SHIRT_NUMBER = 99;

export const DEFAULT_STYLE: Readonly<CharacterStyle> = {
  hair: "short", headwear: "none", skin: 1, hairColor: 0, hatColor: 0, headSize: 1, build: 1, number: 10,
  eyewear: "none", facialHair: "none", wristbands: false, eyewearColor: 0, wristColor: 0, bootsColor: 0,
};

const clamp = (v: number, [lo, hi]: readonly [number, number]): number => Math.min(hi, Math.max(lo, v));
const index = (v: unknown, n: number): number => (Number.isInteger(v) && (v as number) >= 0 && (v as number) < n ? (v as number) : 0);

/** A valid style from untrusted data (storage): unknown fields fall back to the default. */
export function sanitizeStyle(raw: unknown): CharacterStyle {
  const r = (typeof raw === "object" && raw !== null ? raw : {}) as Partial<Record<keyof CharacterStyle, unknown>>;
  const fx = THEME.fx;
  return {
    hair: HAIR_STYLES.includes(r.hair as HairStyle) ? (r.hair as HairStyle) : DEFAULT_STYLE.hair,
    headwear: HEADWEAR.includes(r.headwear as Headwear) ? (r.headwear as Headwear) : DEFAULT_STYLE.headwear,
    skin: index(r.skin, fx.skinTones.length),
    hairColor: index(r.hairColor, fx.hairColors.length),
    hatColor: index(r.hatColor, fx.hatColors.length),
    headSize: clamp(Number.isFinite(r.headSize) ? (r.headSize as number) : 1, HEAD_SIZE_RANGE),
    build: clamp(Number.isFinite(r.build) ? (r.build as number) : 1, BUILD_RANGE),
    number: Number.isInteger(r.number) ? clamp(r.number as number, [0, MAX_SHIRT_NUMBER]) : DEFAULT_STYLE.number,
    eyewear: EYEWEAR.includes(r.eyewear as Eyewear) ? (r.eyewear as Eyewear) : DEFAULT_STYLE.eyewear,
    facialHair: FACIAL_HAIR.includes(r.facialHair as FacialHair) ? (r.facialHair as FacialHair) : DEFAULT_STYLE.facialHair,
    wristbands: r.wristbands === true,
    eyewearColor: index(r.eyewearColor, fx.eyewearColors.length),
    wristColor: index(r.wristColor, fx.gearColors.length),
    bootsColor: index(r.bootsColor, fx.bootColors.length),
  };
}

/** A varied but stable style for a bot/NPC from an integer seed. */
export function styleFromSeed(seed: number): CharacterStyle {
  // 32-bit integer mixing (Math.imul): a plain double multiply loses the low
  // bits past 2^53 and skews every pick toward 0.
  let h = Math.imul(seed | 0, 0x9e3779b1) >>> 0;
  const next = (n: number): number => {
    h = Math.imul(h ^ (h >>> 15), 0x85ebca6b) >>> 0;
    h = Math.imul(h ^ (h >>> 13), 0xc2b2ae35) >>> 0;
    h = (h ^ (h >>> 16)) >>> 0;
    return h % n;
  };
  const fx = THEME.fx;
  const span = (r: readonly [number, number]): number => r[0] + ((next(1000) / 999) * (r[1] - r[0]));
  return {
    hair: HAIR_STYLES[next(HAIR_STYLES.length)],
    headwear: next(4) === 0 ? HEADWEAR[1 + next(2)] : "none",
    skin: next(fx.skinTones.length),
    hairColor: next(fx.hairColors.length),
    hatColor: next(fx.hatColors.length),
    headSize: span(HEAD_SIZE_RANGE),
    build: span(BUILD_RANGE),
    number: 1 + next(MAX_SHIRT_NUMBER),
    eyewear: next(5) === 0 ? EYEWEAR[1 + next(3)] : "none",
    facialHair: next(4) === 0 ? FACIAL_HAIR[1 + next(3)] : "none",
    wristbands: next(3) === 0,
    eyewearColor: next(fx.eyewearColors.length),
    wristColor: next(fx.gearColors.length),
    bootsColor: next(fx.bootColors.length),
  };
}
