import type { GameMode } from "../ui/types";
import { t } from "../i18n/t";

// Mirror of sim/lobby.go (docs/design/lobby.md §2). Verified at load against
// futenis.layout().lobbyPads / lobbyPadRadius (layout-check.ts). CHANGE TOGETHER.
export const LOBBY_PAD_COUNT = 3;
/** Flat [x0, z0, x1, z1, x2, z2] of the pad centers (Y = 0). */
export const LOBBY_PADS: readonly number[] = [-6, -4, 0, -6, 6, -4];
export const LOBBY_PAD_RADIUS = 1.5;
/** Default walkable half extents (Config LobbyHalfX/Z, lobby.md §3). */
export const LOBBY_HALF_X = 16;
export const LOBBY_HALF_Z = 18;

/** Config scalar indices appended after cfgFeedValidBounce = 71 (lobby.md §3). */
export const LobbyCfg = { HoldTime: 72, HalfX: 73, HalfZ: 74 } as const;

/** Pad index -> mode it starts (lobby.md §6). */
export const PAD_MODES: readonly GameMode[] = ["playground", "1v1", "2v2"];
export const PAD_LABELS: readonly string[] = [
  t("Entrenamiento · Lanzapelotas", "Training · Ball machine"),
  t("1v1 contra el bot", "1v1 vs the bot"),
  t("2v2 contra bots", "2v2 vs bots"),
];

export const padX = (i: number): number => LOBBY_PADS[i * 2];
export const padZ = (i: number): number => LOBBY_PADS[i * 2 + 1];
