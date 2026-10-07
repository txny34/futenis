/** Lobby menu (Esc in the lobby): difficulty + a simple item list. Pure builder. */
import { button, el, segmented } from "./dom";
import { DIFFICULTIES, panel } from "./screens";
import type { Difficulty } from "./types";

/** One row of the menu; add future entries to the list in main.ts. */
export interface LobbyMenuItem {
  readonly label: string;
  readonly run: () => void;
  readonly ghost?: boolean;
}

export interface LobbyMenuModel {
  readonly difficulty: Difficulty;
  readonly onDifficulty: (d: Difficulty) => void;
  readonly items: readonly LobbyMenuItem[];
}

export function lobbyMenu(m: LobbyMenuModel, close: () => void): HTMLElement {
  const nav = el("nav", "fx-stack", "", { "aria-label": "Menú del lobby" });
  nav.append(...m.items.map((i) => button(i.label, i.run, i.ghost ? "fx-btn fx-btn--ghost" : "fx-btn")));
  nav.append(button("Volver", close, "fx-btn fx-btn--ghost"));
  return panel("Menú", segmented("Dificultad", DIFFICULTIES, m.difficulty, m.onDifficulty), nav);
}
