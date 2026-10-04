import { EventKind, Mode, SimEvent, Snap } from "./snapshot";

// Entrenamiento HUD (docs/design/training.md §5): big RACHA counter + MEJOR,
// "¡+1!" on every legal return, "RACHA: N" when a streak > 0 breaks, next-feed
// countdown (SnapFeedIn, hidden at -1) and the on-demand launch hint.
// Built once; text only changes on change.
const POP_MS = 900;
const ANIM_CLASS = "is-live";
const PULSE_CLASS = "is-pulse";
const UNSET = -1;
const TENTHS = 10;
const OK_TEXT = "¡+1!";

function node(tag: "div" | "span" | "b", cls: string, text = ""): HTMLElement {
  const e = document.createElement(tag);
  e.className = cls;
  e.textContent = text;
  return e;
}

export class TrainingHud {
  readonly root = node("div", "");
  readonly pop = node("div", "");
  private readonly streak = node("b", "train__streak", "0");
  private readonly best = node("span", "train__best", "MEJOR 0");
  private readonly next = node("div", "train__next");
  private readonly hint = node("div", "train__hint");
  private lastStreak = UNSET;
  private lastBest = UNSET;
  private lastNext = UNSET - 1;
  private popUntil = 0;
  private onDemand = false;

  constructor(launchKey: string) {
    this.root.id = "train";
    this.pop.id = "train-pop";
    this.root.hidden = true;
    this.pop.hidden = true;
    this.next.hidden = true;
    this.hint.textContent = `${launchKey}: lanzar bola`;
    this.hint.hidden = true;
    this.root.append(node("span", "train__label", "RACHA"), this.streak, this.best, this.next, this.hint);
  }

  /** FeedOnDemand of the current session (read once at match start). */
  setOnDemand(v: boolean): void {
    this.onDemand = v;
    this.hint.hidden = !v;
  }

  reset(): void {
    this.lastStreak = UNSET;
    this.lastBest = UNSET;
    this.lastNext = UNSET - 1;
    this.pop.hidden = true;
  }

  onEvent(e: SimEvent, nowMs: number): void {
    if (e.kind === EventKind.ReturnOK) this.popup(OK_TEXT, "ok", nowMs);
    else if (e.kind === EventKind.ReturnFail && this.lastStreak > 0) this.popup(`RACHA: ${this.lastStreak}`, "fail", nowMs);
  }

  update(snap: Float64Array, nowMs: number): void {
    const on = snap[Snap.Mode] === Mode.Playground;
    if (this.root.hidden === on) this.root.hidden = !on;
    if (!this.pop.hidden && nowMs > this.popUntil) this.pop.hidden = true;
    if (!on) return;
    this.updateStreak(snap[Snap.Streak], snap[Snap.BestStreak]);
    this.updateNext(snap[Snap.FeedIn]);
  }

  private updateStreak(streak: number, best: number): void {
    if (streak !== this.lastStreak) {
      const grew = streak > this.lastStreak && this.lastStreak !== UNSET;
      this.lastStreak = streak;
      this.streak.textContent = String(streak);
      if (grew) {
        this.streak.classList.remove(PULSE_CLASS);
        void this.streak.offsetWidth; // restart the CSS animation
        this.streak.classList.add(PULSE_CLASS);
      }
    }
    if (best === this.lastBest) return;
    this.lastBest = best;
    this.best.textContent = `MEJOR ${best}`;
  }

  /** Countdown in tenths of a second; -1 (waiting for demand / ball live) hides it. */
  private updateNext(feedIn: number): void {
    const t = feedIn < 0 ? UNSET : Math.ceil(feedIn * TENTHS);
    if (t === this.lastNext) return;
    this.lastNext = t;
    this.next.hidden = t === UNSET || this.onDemand;
    if (t !== UNSET) this.next.textContent = `Próxima bola ${(t / TENTHS).toFixed(1)} s`;
  }

  private popup(text: string, tone: string, nowMs: number): void {
    this.popUntil = nowMs + POP_MS;
    this.pop.dataset.tone = tone;
    this.pop.textContent = text;
    this.pop.classList.remove(ANIM_CLASS);
    this.pop.hidden = false;
    void this.pop.offsetWidth; // restart the CSS animation
    this.pop.classList.add(ANIM_CLASS);
  }
}
