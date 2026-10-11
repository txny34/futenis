import { button, el } from "./dom";
import { panel } from "./screens";
import { t } from "../i18n/t";

// "Jugadores en el lobby": hide a player for yourself, or report their name
// (the server hides it for everyone once players from two networks report it).
// Names are set as text (never HTML).
export interface LobbyPlayer { readonly id: number; readonly name: string; readonly hidden: boolean }

export interface LobbyPlayersModel {
  list(): LobbyPlayer[];
  setHidden(id: number, on: boolean): void;
  report(id: number): void;
}

const reported = new Set<number>(); // this page: one report per player

export function lobbyPlayersView(m: LobbyPlayersModel, back: () => void): HTMLElement {
  const rows = el("ul", "fx-players");
  const draw = (focusKey = ""): void => {
    rows.replaceChildren();
    const players = m.list();
    if (players.length === 0) rows.append(el("li", "fx-players__empty", t("No hay nadie más en el lobby.", "Nobody else is in the lobby.")));
    for (const p of players) rows.append(row(m, p, draw));
    // Keep keyboard focus on the row just used (a reported row's button is disabled: its hide button then).
    const target = rows.querySelector<HTMLButtonElement>(`[data-key="${focusKey}"]:not(:disabled)`)
      ?? rows.querySelector<HTMLButtonElement>(`[data-key="${focusKey.split(":")[0]}:hide"]`);
    target?.focus({ preventScroll: true });
  };
  draw();
  const hint = el("p", "fx-report__hint", t(
    "Ocultar es solo para vos y dura mientras esa persona siga conectada. Si varios reportan un nombre, se reemplaza por \"Jugador\".",
    "Hiding is just for you and lasts while that player stays connected. If several players report a name, it is replaced with \"Jugador\".",
  ));
  return panel(t("Jugadores en el lobby", "Players in the lobby"), rows, hint, button(t("Volver", "Back"), back, "fx-btn fx-btn--ghost"));
}

function row(m: LobbyPlayersModel, p: LobbyPlayer, redraw: (focusKey: string) => void): HTMLElement {
  const li = el("li", "fx-players__row");
  const name = el("span", "fx-players__name", p.name || t("Jugador", "Player"));
  const hide = button(p.hidden ? t("Mostrar", "Show") : t("Ocultar", "Hide"), () => { m.setHidden(p.id, !p.hidden); redraw(`${p.id}:hide`); }, "fx-btn fx-btn--ghost");
  hide.dataset.key = `${p.id}:hide`;
  const done = reported.has(p.id);
  const report = button(done ? t("Reportado", "Reported") : t("Reportar nombre", "Report name"), () => {
    m.report(p.id);
    reported.add(p.id);
    redraw(`${p.id}:report`);
  }, "fx-btn fx-btn--ghost");
  report.dataset.key = `${p.id}:report`;
  report.disabled = done;
  li.append(name, hide, report);
  return li;
}
