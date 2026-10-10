// Lobby decor tokens split out of theme.ts to keep files under the 200-line limit.
// Colors stay here or in theme.ts — never elsewhere.
export interface LobbyDecorTheme {
  /** Floor canvas (CSS): tile base, tile highlight, seam glow, center logo. */
  readonly floorTile: string;
  readonly floorTileHi: string;
  readonly floorSeam: string;
  readonly floorLogo: string;
  /** Neon arches / tubes cycle between these three. */
  readonly neon: readonly [number, number, number];
  readonly wallPanel: number;
  readonly wallTrim: number;
  readonly plinth: number;
  /** Glowing teal band around each pad platform. */
  readonly padRim: number;
  /** Big floating pad label (CSS glow under the text). */
  readonly padLabelText: string;
  /** Showcase court lighting. */
  readonly showcaseLight: number;
  /** Giant screen + posters (CSS). */
  readonly screenBg: string;
  readonly screenText: string;
  readonly screenAccentA: string;
  readonly screenAccentB: string;
  readonly posterBg: readonly string[];
  readonly dust: number;
  readonly beam: number;
  readonly confetti: readonly number[];
  readonly hemiSky: number;
  readonly hemiGround: number;
  /** NPC looks: shirt colors (shorts/skin/hair/boots reuse crowd tokens). */
  readonly npcShirts: readonly number[];
  /** Shared online lobby: name tag over other players (CSS colors for the canvas). */
  readonly nameTagText: string;
  readonly nameTagStroke: string;
  /** Zona 2 · Salón de la fama (lobby/zone2): calm boards, podium medals, statue. */
  readonly hall: HallTheme;
}

export interface HallTheme {
  /** Board canvas (CSS): background, title, muted placeholder, own-row fill + text. */
  readonly boardBg: string;
  readonly boardTitle: string;
  readonly boardMuted: string;
  readonly ownRow: string;
  readonly ownText: string;
  /** Board frame + podium medal trims (numbers): gold, silver, bronze. */
  readonly frame: number;
  readonly medals: readonly [number, number, number];
  readonly podium: number;
  readonly light: number;
  /** Floor arrow sign (CSS). */
  readonly signText: string;
}

export const LOBBY_DECOR: LobbyDecorTheme = {
  floorTile: "#161c3a",
  floorTileHi: "#1f2752",
  floorSeam: "#4b2fa3",
  floorLogo: "#19e3ff",
  neon: [0xb04dff, 0x19e3ff, 0xff4fa3],
  wallPanel: 0x120f2e,
  wallTrim: 0x7a4dff,
  plinth: 0x1a1d3d,
  padRim: 0x2bffb0,
  padLabelText: "#ffffff",
  showcaseLight: 0xdfe9ff,
  screenBg: "#07051a",
  screenText: "#eef6ff",
  screenAccentA: "#b04dff",
  screenAccentB: "#19e3ff",
  posterBg: ["#2a0f4a", "#0b2a4a", "#4a0f33"],
  dust: 0xc9b8ff,
  beam: 0x9b6bff,
  confetti: [0xe6ff1f, 0xff4fa3, 0x19e3ff, 0xb04dff, 0xff5a4e],
  hemiSky: 0x6a3dd8,
  hemiGround: 0x0a1a3a,
  npcShirts: [0x7a4dff, 0x19c6a0, 0xffb020, 0x3b7bff, 0xff4fa3],
  nameTagText: "#eef6ff",
  nameTagStroke: "#0b0f26",
  hall: {
    boardBg: "#0b0f26",
    boardTitle: "#ffd36b",
    boardMuted: "#55628a",
    ownRow: "#1d3fa8",
    ownText: "#eef6ff",
    frame: 0x2a3160,
    medals: [0xd9a93a, 0xaeb7c6, 0xb0703a],
    podium: 0x1c2250,
    light: 0xfff1d6,
    signText: "#ffd36b",
  },
};
