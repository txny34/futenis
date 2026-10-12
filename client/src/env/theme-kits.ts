// Team kits (shirt, partner shorts, trim, number ink): the base the
// commercial kit skins will swap. Part of THEME (theme.ts).

export interface Kit {
  readonly shirt: number;
  /** Shorts of slot 0 / slot 1 inside a team (2v2 partners tell apart). */
  readonly shorts: readonly [number, number];
  /** Collar and socks. */
  readonly trim: number;
  /** Name and number on the back. */
  readonly ink: number;
}

const WHITE = 0xf2f2f2;
const CHARCOAL = 0x2b2f36;
const NAVY = 0x0b1a2e;

export function makeKits(teamA: number, teamB: number): readonly [Kit, Kit] {
  return [
    { shirt: teamA, shorts: [WHITE, CHARCOAL], trim: WHITE, ink: WHITE },
    { shirt: teamB, shorts: [WHITE, CHARCOAL], trim: NAVY, ink: NAVY }, // dark ink: white is unreadable on cyan
  ];
}
