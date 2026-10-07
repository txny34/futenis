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
  /** Entrenamiento ball machine (view/machine.ts). */
  readonly machine: { readonly body: number; readonly barrel: number; readonly trim: number; readonly flash: number };
  /** Lobby (docs/design/lobby.md §7): floor, neon border, pad colors (training/1v1/2v2), hold ring. */
  readonly lobbyFloor: number;
  readonly lobbyEdge: number;
  readonly padTraining: number;
  readonly pad1v1: number;
  readonly pad2v2: number;
  readonly padFill: number;
  /** Pad sign canvas (CSS colors): background and text. */
  readonly lobbySignBg: string;
  readonly lobbySignText: string;
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
  /** Entrenamiento minimap: machine zone (pink), bounce zone (cyan), court. */
  readonly trainMachine: string;
  readonly trainBounce: string;
  readonly trainCourt: string;
  readonly trainLine: string;
}

export const THEME: Theme = {
  skyTop: 0x02040d,
  skyHorizon: 0x10224a,
  fog: 0x070d22,
  courtFloor: 0x2c3354, // slate blue-grey like the reference: the neon ball and white lines pop
  courtLines: 0xf4fbff,
  apron: 0x1a1f38,
  glass: 0x9fe8ff,
  glassEdge: 0x2a3142, // dark metal panel frames
  mesh: 0x07090f,
  metal: 0x161b26,
  netMesh: 0x05080d,
  netBand: 0x3b7bff,
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
  machine: { body: 0x1b2340, barrel: 0x0b0f1c, trim: 0xff4fa3, flash: 0xff4fa3 },
  lobbyFloor: 0x141a33,
  lobbyEdge: 0x19e3ff,
  padTraining: 0xff4fa3,
  pad1v1: 0xff5a4e,
  pad2v2: 0x19e3ff,
  padFill: 0xe6ff1f,
  lobbySignBg: "rgba(5, 9, 20, 0.82)",
  lobbySignText: "#eef6ff",
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
    trainMachine: "#ff4fa3",
    trainBounce: "#19e3ff",
    trainCourt: "#2c3354",
    trainLine: "rgba(244, 251, 255, 0.75)",
  },
};

/** CSS custom properties mirroring the HUD tokens (single source of truth). */
export function applyThemeCss(root: HTMLElement, t: HudTheme = THEME.hud): void {
  const vars: ReadonlyArray<readonly [string, string]> = [
    ["--hud-card", t.card], ["--hud-border", t.cardBorder], ["--hud-text", t.text],
    ["--hud-muted", t.muted], ["--hud-accent", t.accent], ["--hud-warn", t.warn],
    ["--hud-ring", t.ring], ["--hud-team0", t.team0], ["--hud-team1", t.team1],
    ["--train-machine", t.trainMachine], ["--train-bounce", t.trainBounce],
    ["--train-court", t.trainCourt], ["--train-line", t.trainLine],
  ];
  for (const [k, v] of vars) root.style.setProperty(k, v);
}
