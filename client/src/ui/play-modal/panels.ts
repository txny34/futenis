import { keyHint, moveHint } from "../key-hints";
import { loadHistory, MatchRecord } from "../../profile/history";
import { el } from "../dom";
import { t } from "../../i18n/t";

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
    el("td", "", r.ranked ? t("Clasificación", "Ranked") : t("Casual", "Casual")),
    el("td", "fx-hist__result", r.won ? t("Victoria", "Win") : t("Derrota", "Loss")),
    el("td", "", `${r.games0} – ${r.games1}`),
    el("td", "fx-hist__delta", deltaText(r)),
  );
  return tr;
}

export function historyPanel(): HTMLElement {
  const list = loadHistory();
  const box = el("div", "fx-play__sheet-body");
  if (list.length === 0) {
    box.append(el("p", "fx-note", t("Todavía no jugaste partidos. Elegí un modo y arrancá.", "You haven't played any matches yet. Pick a mode and go.")));
    return box;
  }
  const table = el("table", "fx-hist");
  const head = el("tr");
  const columns: ReadonlyArray<readonly [string, string]> = [
    ["Fecha", "Date"], ["Modo", "Mode"], ["Tipo", "Type"], ["Resultado", "Result"], ["Juegos", "Games"], ["ELO", "ELO"],
  ];
  for (const [es, en] of columns) head.append(el("th", "", t(es, en)));
  const body = el("tbody");
  body.append(...list.map(historyRow));
  const thead = el("thead");
  thead.append(head);
  table.append(thead, body);
  box.append(table, el("p", "fx-note", t(
    `Últimos ${list.length} partidos (se guardan en este navegador).`,
    `Last ${list.length} matches (saved in this browser).`,
  )));
  return box;
}

const RULES: ReadonlyArray<readonly [string, string]> = [
  ["Fútbol-tenis en cancha de pádel: se juega con el pie y valen las paredes.",
    "Football-tennis on a padel court: played with the foot, and walls are in play."],
  ["1v1: un toque y al otro lado. 2v2: hasta 3 toques por equipo, nunca el mismo dos seguidas (click der. rápido = pase).",
    "1v1: one touch and over. 2v2: up to 3 touches per team, never the same player twice (quick right-click = pass)."],
  ["Al mejor de 3 sets. Cada set a 5 puntos, ganando por 2 (en 6-6, el que hace el 7 gana).",
    "Best of 3 sets. Each set to 5 points, win by 2 (at 6-6, the next point wins)."],
  ["Casual: suma XP y victorias. Clasificación: además mueve tu ELO y da 1.5x XP.",
    "Casual: earns XP and wins. Ranked: also moves your ELO and gives 1.5x XP."],
  ["El nivel del bot se ajusta solo a tu ELO de esa cola.",
    "The bot's level adjusts itself to your ELO in that queue."],
];
const controls = (): ReadonlyArray<readonly [string, string]> => [
  [moveHint(), t("Moverse", "Move")], ["Mouse", t("Cámara y mira", "Camera and aim")],
  [t("Click izq.", "Left click"), t("Golpe", "Strike")], [t("Click der.", "Right click"), t("Globo", "Lob")],
  [keyHint("jump"), t("Salto", "Jump")], [keyHint("dash"), "Dash"], [t("Rueda", "Scroll"), "Zoom"],
  [keyHint("free"), t("Liberar el mouse", "Release the mouse")],
  ["Esc", t("Menú / pausa", "Menu / pause")],
];

export function infoPanel(): HTMLElement {
  const box = el("div", "fx-play__sheet-body");
  const rules = el("ul", "fx-info__rules");
  rules.append(...RULES.map(([es, en]) => el("li", "", t(es, en))));
  const keys = el("dl", "fx-info__keys");
  for (const [k, v] of controls()) keys.append(el("dt", "fx-chip", k), el("dd", "", v));
  box.append(el("h4", "fx-info__h", t("Reglas", "Rules")), rules, el("h4", "fx-info__h", t("Controles", "Controls")), keys);
  return box;
}
