// Gameplay feedback colors (timing ring, impacts, overlays, hit flash,
// character details). Part of THEME (theme.ts) like theme-lobby.ts.

import { CB, COLORBLIND } from "./theme-cb";

export interface FxTheme {
  /** Timing ring by sim.Quality: perfect / good / forced. */
  readonly quality: readonly [number, number, number];
  readonly ringTarget: number;
  readonly landing: number;
  readonly splat: number;
  /** Impact ring: perfect / good / other. */
  readonly impact: readonly [number, number, number];
  readonly serveBox: number;
  readonly reticle: number;
  readonly perfectFlash: number;
  readonly eyeWhite: number;
  readonly eyePupil: number;
  /** Mouth atlas (character/mouth-atlas.ts): lips/inside, teeth, tongue. */
  readonly mouth: number;
  readonly teeth: number;
  readonly tongue: number;
  /** Character customizer palettes (character/style.ts): skin, hair, headwear. */
  readonly skinTones: readonly number[];
  readonly hairColors: readonly number[];
  readonly hatColors: readonly number[];
  /** Character rim light (arena glow on silhouettes), iris, sneaker sole/laces. */
  readonly rim: number;
  readonly iris: number;
  readonly sole: number;
}

export const FX: FxTheme = {
  // colorblind: yellow / sky / purple stay apart for protan, deutan and tritan eyes
  quality: COLORBLIND ? [CB.yellow, CB.sky, CB.purple] : [0xffd23f, 0x19e3ff, 0xff7a6b],
  ringTarget: 0xffffff,
  landing: 0xffffff,
  splat: 0xff3d6e,
  impact: COLORBLIND ? [CB.yellow, CB.sky, 0xffffff] : [0xffd23f, 0x19e3ff, 0xffffff],
  serveBox: 0xffd23f,
  reticle: 0xffffff,
  perfectFlash: 0xffc23f,
  eyeWhite: 0xffffff,
  eyePupil: 0x111111,
  mouth: 0x3a1418,
  teeth: 0xfdf8f0,
  tongue: 0xe8607a,
  skinTones: [0xf6d2b8, 0xe0ac7e, 0xc68a5e, 0x9a6440, 0x6e4529, 0x4a2c1a],
  hairColors: [0x2b1d14, 0x5a3720, 0xa8642f, 0xe0b45a, 0xd8d4cc, 0x1b1e2b],
  hatColors: [0x19e3ff, 0xff4fa3, 0xe6ff1f, 0xffd23f, 0xf2f2f2, 0x1d2438],
  rim: 0x9fe8ff,
  iris: 0x4a7fd0,
  sole: 0xf4f6fa,
};
