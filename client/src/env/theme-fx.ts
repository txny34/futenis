// Gameplay feedback colors (timing ring, impacts, overlays, hit flash,
// character details). Part of THEME (theme.ts) like theme-lobby.ts.

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
  readonly aimRing: number;
  readonly perfectFlash: number;
  /** Shorts of slot 0 / slot 1 inside a team (2v2). */
  readonly partnerShorts: readonly [number, number];
  readonly eyeWhite: number;
  readonly eyePupil: number;
}

export const FX: FxTheme = {
  quality: [0xffd23f, 0x19e3ff, 0xff7a6b],
  ringTarget: 0xffffff,
  landing: 0xffffff,
  splat: 0xff3d6e,
  impact: [0xffd23f, 0x19e3ff, 0xffffff],
  serveBox: 0xffd23f,
  reticle: 0xffffff,
  aimRing: 0xffd23f,
  perfectFlash: 0xffc23f,
  partnerShorts: [0xf2f2f2, 0x2b2f36],
  eyeWhite: 0xffffff,
  eyePupil: 0x111111,
};
