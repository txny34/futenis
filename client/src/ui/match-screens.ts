/** Match-flow overlays: VS intro before the first serve and the VICTORIA/DERROTA result. */
import { button, el } from "./dom";

export const VS_INTRO_MS = 2000;
const TEAM_NAMES = ["Local", "Visita"] as const;
const LEAVE_CLASS = "is-leaving";
const LEAVE_MS = 300;

export interface ResultInfo {
  readonly won: boolean;
  readonly games0: number;
  readonly games1: number;
}

export interface ResultActions { rematch: () => void; lobby: () => void }

function team(i: 0 | 1, members: number): HTMLElement {
  const t = el("div", `fx-vs__team fx-vs__team--${i}`);
  const sils = el("div", "fx-vs__sils", "", { "aria-hidden": "true" });
  for (let k = 0; k < members; k++) sils.append(el("span", "fx-sil"));
  t.append(sils, el("span", "fx-vs__name", TEAM_NAMES[i]));
  return t;
}

/** Owns one fixed layer above the HUD; only one screen at a time. */
export class MatchScreens {
  private readonly layer = el("div", "fx-flow");
  private leaveTimer = 0;

  constructor(parent: HTMLElement) {
    this.layer.hidden = true;
    parent.append(this.layer);
  }

  /** members: players per team (1 in 1v1, 2 in 2v2). Fades out by itself after VS_INTRO_MS. */
  showIntro(members: number, subtitle: string): void {
    const vs = el("div", "fx-vs");
    vs.append(team(0, members), el("span", "fx-vs__vs", "VS"), team(1, members));
    this.mount(vs, el("p", "fx-vs__sub", subtitle));
    this.leaveTimer = window.setTimeout(() => this.leave(), VS_INTRO_MS - LEAVE_MS);
  }

  showResult(r: ResultInfo, a: ResultActions): void {
    const card = el("section", `fx-result fx-result--${r.won ? "win" : "lose"}`, "", { role: "dialog", "aria-modal": "true" });
    const actions = el("div", "fx-actions");
    actions.append(button("Revancha", a.rematch, "fx-btn fx-btn--hero"), button("Volver al lobby", a.lobby, "fx-btn fx-btn--ghost"));
    card.append(
      el("h2", "fx-result__title", r.won ? "VICTORIA" : "DERROTA"),
      el("p", "fx-result__score", `${TEAM_NAMES[0]} ${r.games0} – ${r.games1} ${TEAM_NAMES[1]}`),
      el("p", "fx-result__label", "Juegos"),
      actions,
    );
    this.mount(card);
    const first = card.querySelector<HTMLButtonElement>("button");
    if (first) first.focus({ preventScroll: true });
  }

  hide(): void {
    window.clearTimeout(this.leaveTimer);
    this.layer.classList.remove(LEAVE_CLASS);
    this.layer.replaceChildren();
    this.layer.hidden = true;
  }

  private mount(...nodes: HTMLElement[]): void {
    this.hide();
    this.layer.append(...nodes);
    this.layer.hidden = false;
  }

  private leave(): void {
    this.layer.classList.add(LEAVE_CLASS);
    this.leaveTimer = window.setTimeout(() => this.hide(), LEAVE_MS);
  }
}
