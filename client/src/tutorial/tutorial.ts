import { t } from "../i18n/t";
import { SimEvent } from "../snapshot";
import { TUTORIAL_STEPS, TutorialProgress } from "./steps";

// Tutorial overlay: step card (title, instruction, progress pips) over the
// ball machine. Touches the DOM only when the progress changes.
const DONE_KEY = "futenis.tutorial.done";
const DONE_TITLE = t("¡Listo!", "Done!");
const DONE_TEXT = t(
  "Ya sabés lo básico. Esc → «Volver al lobby» y jugar contra bots.",
  "You know the basics now. Esc → \"Back to lobby\" to play against bots.",
);

function node(tag: "div" | "span", cls: string): HTMLElement {
  const e = document.createElement(tag);
  e.className = cls;
  return e;
}

/** True once the player finished (or skipped) the tutorial on this browser. */
export function tutorialDone(): boolean {
  try { return localStorage.getItem(DONE_KEY) === "1"; } catch { return false; }
}

function markDone(): void {
  try { localStorage.setItem(DONE_KEY, "1"); } catch { /* storage blocked: offered again next visit */ }
}

export class Tutorial {
  private readonly root = node("div", "tut");
  private readonly head = node("div", "tut__head");
  private readonly title = node("div", "tut__title");
  private readonly text = node("div", "tut__text");
  private readonly pips = node("div", "tut__pips");
  private readonly progress = new TutorialProgress();
  private active = false;

  constructor(parent: HTMLElement) {
    this.root.hidden = true;
    this.root.setAttribute("role", "status");
    this.root.setAttribute("aria-live", "polite");
    this.root.setAttribute("aria-atomic", "true");
    this.pips.setAttribute("aria-hidden", "true");
    for (let i = 0; i < TUTORIAL_STEPS.length; i++) this.pips.append(node("span", "tut__pip"));
    this.root.append(this.head, this.title, this.text, this.pips);
    parent.append(this.root);
  }

  start(): void {
    this.active = true;
    this.progress.reset();
    this.root.hidden = false;
    this.render();
  }

  /** Leaving the tutorial (back to lobby) counts as seen: never forced again.
   * A match picked before the first load never started it, so it is offered again. */
  stop(): void {
    if (this.active) markDone();
    this.active = false;
    this.root.hidden = true;
  }

  onEvent(e: SimEvent): void {
    if (!this.active || this.progress.done || !this.progress.onEvent(e)) return;
    if (this.progress.done) markDone();
    this.render();
  }

  private render(): void {
    const p = this.progress;
    const step = TUTORIAL_STEPS[p.step];
    const n = TUTORIAL_STEPS.length;
    this.head.textContent = step ? `TUTORIAL ${p.step + 1}/${n}` : "TUTORIAL";
    this.title.textContent = step ? step.title : DONE_TITLE;
    this.text.textContent = step ? withCount(step.text(), p.hits, step.count) : DONE_TEXT;
    this.root.classList.toggle("is-done", !step);
    const pips = this.pips.children;
    for (let i = 0; i < pips.length; i++) {
      pips[i].classList.toggle("is-on", i < p.step);
      pips[i].classList.toggle("is-now", i === p.step);
    }
  }
}

function withCount(text: string, hits: number, count: number): string {
  return count > 1 ? `${text} (${hits}/${count})` : text;
}
