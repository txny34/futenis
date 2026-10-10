import { el } from "../dom";
import { modeArt } from "./art/index";
import { ballIcon, trophyIcon } from "./art/trophy";
import { t } from "../../i18n/t";

// Mode cards of "Elegí un modo". Online (default): casual + ranked against people.
// Bots (side button): casual only — ELO is earned only in ranked online matches.
export type QueueMode = "1v1" | "2v2";
export type PlayKind = "online" | "bots";

export interface PlayPick {
  readonly mode: QueueMode;
  readonly ranked: boolean;
}

export interface CardDef extends PlayPick {
  readonly title: string;
  readonly sub: string;
  /** false: shown but JUGAR answers "Próximamente". */
  readonly available: boolean;
}

const C1 = t("1v1 · CASUAL", "1v1 · CASUAL");
const C2 = t("2v2 · CASUAL", "2v2 · CASUAL");
export const ONLINE_CASUAL: readonly CardDef[] = [
  { mode: "1v1", ranked: false, available: true, title: C1, sub: t("Contra otra persona · sin ELO", "Vs another player · no ELO") },
  { mode: "2v2", ranked: false, available: true, title: C2, sub: t("Online · si faltan jugadores, completan bots", "Online · bots fill empty seats") },
];
export const ONLINE_RANKED: readonly CardDef[] = [
  { mode: "1v1", ranked: true, available: true, title: t("1v1 · CLASIFICACIÓN", "1v1 · RANKED"), sub: t("Suma ELO · requiere cuenta", "Earns ELO · account required") },
  { mode: "2v2", ranked: true, available: true, title: t("2v2 · CLASIFICACIÓN", "2v2 · RANKED"), sub: t("Suma ELO · 4 personas, sin bots", "Earns ELO · 4 people, no bots") },
];
export const BOT_CASUAL: readonly CardDef[] = [
  { mode: "1v1", ranked: false, available: true, title: C1, sub: t("Contra el bot · sin ELO", "Vs the bot · no ELO") },
  { mode: "2v2", ranked: false, available: true, title: C2, sub: t("Vos + bot contra dos bots", "You + bot vs two bots") },
];

export const samePick = (a: PlayPick | null, b: PlayPick): boolean => a !== null && a.mode === b.mode && a.ranked === b.ranked;

export function modeCard(d: CardDef, onPick: (p: PlayPick) => void): HTMLButtonElement {
  const b = el("button", `fx-mode${d.ranked ? " fx-mode--ranked" : " fx-mode--casual"}`, "", { type: "button", role: "radio", "aria-checked": "false" });
  const band = el("span", "fx-mode__band");
  band.append(el("b", "fx-mode__title", d.title));
  const sub = el("span", d.ranked ? "fx-mode__sub fx-mode__pill" : "fx-mode__sub", d.sub);
  b.append(modeArt(d.mode, d.ranked), el("span", "fx-mode__sheen", "", { "aria-hidden": "true" }), band,
    el("span", "fx-mode__check", "", { "aria-hidden": "true" }), sub);
  if (d.ranked) b.append(trophyIcon(`pm-t-${d.mode}`, "fx-trophy fx-mode__trophy"));
  b.addEventListener("click", () => onPick({ mode: d.mode, ranked: d.ranked }));
  return b;
}

/** Labeled section: icon + heading + a row of cards. */
export function cardSection(title: string, ranked: boolean, cards: readonly HTMLElement[]): HTMLElement {
  const s = el("section", `fx-play__section fx-play__section--${ranked ? "ranked" : "casual"}`);
  const row = el("div", "fx-play__cards", "", { role: "radiogroup", "aria-label": title });
  row.append(...cards);
  const h = el("h3", "fx-play__heading");
  h.append(ranked ? trophyIcon("pm-t-head", "fx-trophy fx-play__icon") : ballIcon(), el("span", "fx-play__label", title));
  s.append(h, row);
  return s;
}
