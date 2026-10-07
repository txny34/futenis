import { el } from "../dom";
import { modeArt } from "./art/index";
import { ballIcon, trophyIcon } from "./art/trophy";

// The four mode cards of "Elegí un modo": two casual, two ranked.
export type QueueMode = "1v1" | "2v2";

export interface PlayPick {
  readonly mode: QueueMode;
  readonly ranked: boolean;
}

interface CardDef extends PlayPick {
  readonly title: string;
  readonly sub: string;
}

export const CASUAL_CARDS: readonly CardDef[] = [
  { mode: "1v1", ranked: false, title: "1v1 · CASUAL", sub: "Sin ELO en juego" },
  { mode: "2v2", ranked: false, title: "2v2 · CASUAL", sub: "Vos + bot contra dos bots" },
];
export const RANKED_CARDS: readonly CardDef[] = [
  { mode: "1v1", ranked: true, title: "1v1 · CLASIFICACIÓN", sub: "Suma ELO · 1.5x XP" },
  { mode: "2v2", ranked: true, title: "2v2 · CLASIFICACIÓN", sub: "Suma ELO · 1.5x XP" },
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
