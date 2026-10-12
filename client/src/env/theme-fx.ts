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
  /** Shorts of slot 0 / slot 1 inside a team (2v2). */
  readonly partnerShorts: readonly [number, number];
  readonly eyeWhite: number;
  readonly eyePupil: number;
  /** Mouth atlas (character/mouth-atlas.ts): lips/inside, teeth, tongue. */
  readonly mouth: number;
  readonly teeth: number;
  readonly tongue: number;
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
  partnerShorts: [0xf2f2f2, 0x2b2f36],
  eyeWhite: 0xffffff,
  eyePupil: 0x111111,
  mouth: 0x3a1418,
  teeth: 0xfdf8f0,
  tongue: 0xe8607a,
};
