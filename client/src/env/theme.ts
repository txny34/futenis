// "Arena nocturna neón": every color of the court, stadium and HUD lives here
// so branding/skins can swap the whole look from one place.
// Palette: deep night navy, electric teal court, hot coral (team 0 / neon A)
// and cyan (team 1 / neon B), neon-yellow ball.
export interface Theme {
  readonly skyTop: number;
  readonly skyHorizon: number;
  readonly fog: number;
  readonly courtFloor: number;
  readonly courtLines: number;
  readonly apron: number;
  readonly glass: number;
  readonly glassEdge: number;
  readonly mesh: number;
  readonly metal: number;
  readonly netMesh: number;
  readonly netBand: number;
  readonly neonA: number;
  readonly neonB: number;
  readonly ball: number;
  readonly ballEmissive: number;
  readonly standConcrete: number;
  readonly standEdge: number;
  readonly crowdSkin: readonly number[];
  readonly crowdBody: readonly number[];
  readonly truss: number;
  readonly lampHead: number;
  readonly lightKey: number;
  readonly lightSpot: number;
  readonly hemiSky: number;
  readonly hemiGround: number;
  readonly ledBg: string;
  readonly ledText: string;
  readonly ledAccent: string;
  readonly team: readonly [number, number];
  readonly hud: HudTheme;
}

export interface HudTheme {
  readonly card: string;
  readonly cardBorder: string;
  readonly text: string;
  readonly muted: string;
  readonly accent: string;
  readonly warn: string;
  readonly ring: string;
  readonly team0: string;
  readonly team1: string;
}

export const THEME: Theme = {
  skyTop: 0x02040d,
  skyHorizon: 0x10224a,
  fog: 0x070d22,
  courtFloor: 0x0c5560, // darker teal: the neon ball and white lines pop
  courtLines: 0xf4fbff,
  apron: 0x0a1630,
  glass: 0x9fe8ff,
  glassEdge: 0xd8f6ff,
  mesh: 0x1a2130,
  metal: 0x161b26,
  netMesh: 0x05080d,
  netBand: 0xffffff,
  neonA: 0xff5a4e,
  neonB: 0x19e3ff,
  ball: 0xe6ff1f,
  ballEmissive: 0x8a9c00,
  standConcrete: 0x141c30,
  standEdge: 0x22304f,
  crowdSkin: [0xf1c6a5, 0xd59a72, 0x9a6440, 0x5c3a24],
  crowdBody: [0x1d2438, 0x2a3350, 0x3a3f55, 0x141a2c],
  truss: 0x0e1322,
  lampHead: 0xf2f7ff,
  lightKey: 0xe8f1ff,
  lightSpot: 0xcfe6ff,
  hemiSky: 0x2a3d72,
  hemiGround: 0x05070d,
  ledBg: "#050914",
  ledText: "#19e3ff",
  ledAccent: "#ff5a4e",
  team: [0xff5a4e, 0x19e3ff],
  hud: {
    card: "rgba(8, 16, 38, 0.62)",
    cardBorder: "rgba(25, 227, 255, 0.35)",
    text: "#eef6ff",
    muted: "#8ea3c7",
    accent: "#19e3ff",
    warn: "#ff5a4e",
    ring: "#e6ff1f",
    team0: "#ff5a4e",
    team1: "#19e3ff",
  },
};

/** CSS custom properties mirroring the HUD tokens (single source of truth). */
export function applyThemeCss(root: HTMLElement, t: HudTheme = THEME.hud): void {
  const vars: ReadonlyArray<readonly [string, string]> = [
    ["--hud-card", t.card], ["--hud-border", t.cardBorder], ["--hud-text", t.text],
    ["--hud-muted", t.muted], ["--hud-accent", t.accent], ["--hud-warn", t.warn],
    ["--hud-ring", t.ring], ["--hud-team0", t.team0], ["--hud-team1", t.team1],
  ];
  for (const [k, v] of vars) root.style.setProperty(k, v);
}
