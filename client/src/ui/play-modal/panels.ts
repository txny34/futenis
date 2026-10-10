import { keyHint, moveHint } from "../key-hints";
import { loadHistory, MatchRecord } from "../../profile/history";
import { el } from "../dom";

// Sub-panels of "Elegí un modo": real local history and a short rules/controls sheet.
const DATE_FMT = new Intl.DateTimeFormat("es-AR", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" });

function deltaText(r: MatchRecord): string {
  if (!r.ranked) return "—";
  return r.eloDelta > 0 ? `+${r.eloDelta}` : String(r.eloDelta);
}

function historyRow(r: MatchRecord): HTMLElement {
  const tr = el("tr", r.won ? "is-win" : "is-loss");
  tr.append(
    el("td", "", DATE_FMT.format(r.at)),
    el("td", "", r.queue),
    el("td", "", r.ranked ? "Clasificación" : "Casual"),
    el("td", "fx-hist__result", r.won ? "Victoria" : "Derrota"),
    el("td", "", `${r.games0} – ${r.games1}`),
    el("td", "fx-hist__delta", deltaText(r)),
  );
  return tr;
}

export function historyPanel(): HTMLElement {
  const list = loadHistory();
  const box = el("div", "fx-play__sheet-body");
  if (list.length === 0) {
    box.append(el("p", "fx-note", "Todavía no jugaste partidos. Elegí un modo y arrancá."));
    return box;
  }
  const table = el("table", "fx-hist");
  const head = el("tr");
  for (const h of ["Fecha", "Modo", "Tipo", "Resultado", "Juegos", "ELO"]) head.append(el("th", "", h));
  const body = el("tbody");
  body.append(...list.map(historyRow));
  const thead = el("thead");
  thead.append(head);
  table.append(thead, body);
  box.append(table, el("p", "fx-note", `Últimos ${list.length} partidos (se guardan en este navegador).`));
  return box;
}

const RULES: readonly string[] = [
  "Fútbol-tenis en cancha de pádel: se juega con el pie y valen las paredes.",
  "Un solo toque por equipo antes de devolverla al otro lado.",
  "Puntos como en pádel: 15 · 30 · 40 · ventaja. Un set a 3 juegos.",
  "Casual: suma XP y victorias. Clasificación: además mueve tu ELO y da 1.5x XP.",
  "El nivel del bot se ajusta solo a tu ELO de esa cola.",
];
const controls = (): ReadonlyArray<readonly [string, string]> => [
  [moveHint(), "Moverse"], ["Mouse", "Cámara y mira"], ["Click izq.", "Golpe"], ["Click der.", "Globo"],
  [keyHint("jump"), "Salto"], [keyHint("dash"), "Dash"], ["Rueda", "Zoom"], [keyHint("free"), "Liberar el mouse"],
  ["Esc", "Menú / pausa"],
];

export function infoPanel(): HTMLElement {
  const box = el("div", "fx-play__sheet-body");
  const rules = el("ul", "fx-info__rules");
  rules.append(...RULES.map((t) => el("li", "", t)));
  const keys = el("dl", "fx-info__keys");
  for (const [k, v] of controls()) keys.append(el("dt", "fx-chip", k), el("dd", "", v));
  box.append(el("h4", "fx-info__h", "Reglas"), rules, el("h4", "fx-info__h", "Controles"), keys);
  return box;
}
