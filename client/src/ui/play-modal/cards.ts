import { el } from "../dom";
import { modeArt } from "./art";

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
  { mode: "1v1", ranked: false, title: "1v1 · Casual", sub: "Sin ELO en juego" },
  { mode: "2v2", ranked: false, title: "2v2 · Casual", sub: "Vos + bot contra dos bots" },
];
export const RANKED_CARDS: readonly CardDef[] = [
  { mode: "1v1", ranked: true, title: "1v1 · Clasificación", sub: "Suma ELO · 1.5x XP" },
  { mode: "2v2", ranked: true, title: "2v2 · Clasificación", sub: "Suma ELO · 1.5x XP" },
];

export const samePick = (a: PlayPick | null, b: PlayPick): boolean => a !== null && a.mode === b.mode && a.ranked === b.ranked;

export function modeCard(d: CardDef, onPick: (p: PlayPick) => void): HTMLButtonElement {
  const b = el("button", `fx-mode${d.ranked ? " fx-mode--ranked" : ""}`, "", { type: "button", role: "radio", "aria-checked": "false" });
  const text = el("span", "fx-mode__text");
  text.append(el("b", "fx-mode__title", d.title), el("span", "fx-mode__sub", d.sub));
  b.append(modeArt(d.mode === "1v1" ? 1 : 2), el("span", "fx-mode__check", "", { "aria-hidden": "true" }), text);
  b.addEventListener("click", () => onPick({ mode: d.mode, ranked: d.ranked }));
  return b;
}

/** Labeled section: heading + a row of cards. */
export function cardSection(title: string, cls: string, cards: readonly HTMLElement[]): HTMLElement {
  const s = el("section", `fx-play__section ${cls}`);
  const row = el("div", "fx-play__cards", "", { role: "radiogroup", "aria-label": title });
  row.append(...cards);
  s.append(el("h3", "fx-play__heading", title), row);
  return s;
}
