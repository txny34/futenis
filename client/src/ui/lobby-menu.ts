/** Lobby menu (Esc in the lobby): a simple item list (no difficulty: the bot level is automatic). Pure builder. */
import { button, el } from "./dom";
import { panel } from "./screens";

/** One row of the menu; add future entries to the list in main.ts. */
export interface LobbyMenuItem {
  readonly label: string;
  readonly run: () => void;
  readonly ghost?: boolean;
}

export interface LobbyMenuModel {
  readonly items: readonly LobbyMenuItem[];
}

export function lobbyMenu(m: LobbyMenuModel, close: () => void): HTMLElement {
  const nav = el("nav", "fx-stack", "", { "aria-label": "Menú del lobby" });
  nav.append(...m.items.map((i) => button(i.label, i.run, i.ghost ? "fx-btn fx-btn--ghost" : "fx-btn")));
  nav.append(button("Volver", close, "fx-btn fx-btn--ghost"));
  return panel("Menú", nav);
}
