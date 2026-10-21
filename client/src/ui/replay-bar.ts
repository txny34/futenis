import { t } from "../i18n/t";
import { button, el } from "./dom";

// Replay mini HUD (backlog "Repeticiones"): play/pause, ±10 s, speed, scrubber,
// time, drone and hide. Pure view over ReplayControls (lobby/replay-player.ts).
export type ReplaySpeed = 0.5 | 1 | 2;
const SPEEDS: readonly ReplaySpeed[] = [0.5, 1, 2];
const SKIP_S = 10;
const SCRUB_MAX = 1000;
const HINT_MS = 2000;

export interface ReplayControls {
  togglePause(): void;
  setSpeed(s: ReplaySpeed): void;
  skip(seconds: number): void;
  seekFraction(f: number): void;
  toggleDrone(): void;
  toggleHud(): void;
  exit(): void;
}

export interface ReplayStatus { tick: number; total: number; paused: boolean; speed: ReplaySpeed; drone: boolean; hudHidden: boolean }

const TICK_RATE = 120;
const clock = (ticks: number): string => {
  const s = Math.floor(ticks / TICK_RATE);
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;
};

export class ReplayBar {
  private readonly root = el("div", "fx-replay");
  private readonly hint = el("div", "fx-replay__hint", t("H: mostrar controles", "H: show controls"));
  private readonly play: HTMLButtonElement;
  private readonly drone: HTMLButtonElement;
  private readonly speedBtns: HTMLButtonElement[];
  private readonly scrub = el("input", "fx-replay__scrub", "", { type: "range", min: "0", max: String(SCRUB_MAX), step: "1", "aria-label": t("Posición", "Position") });
  private readonly time = el("span", "fx-replay__time", "0:00 / 0:00");
  private dragging = false;
  private shown = false;
  private blocked = false; // a menu is over the replay: keys belong to it
  private droneOn = false;
  // Last rendered values: the DOM (and any string) is touched only when one changes.
  private lastScrub = -1;
  private lastSec = -1;
  private lastTotal = -1;
  private lastFlags = -1;
  private hintTimer = 0;

  constructor(parent: HTMLElement, private readonly c: ReplayControls) {
    this.play = button("⏸", () => c.togglePause(), "fx-btn fx-replay__btn");
    this.drone = button(t("Dron", "Drone"), () => c.toggleDrone(), "fx-btn fx-replay__btn");
    this.speedBtns = SPEEDS.map((s) => button(`×${String(s).replace(".", ",")}`, () => c.setSpeed(s), "fx-btn fx-replay__btn fx-replay__speed"));
    const back = button("−10s", () => c.skip(-SKIP_S), "fx-btn fx-replay__btn");
    const fwd = button("+10s", () => c.skip(SKIP_S), "fx-btn fx-replay__btn");
    const hide = button(t("Ocultar (H)", "Hide (H)"), () => c.toggleHud(), "fx-btn fx-replay__btn");
    const exit = button("✕", () => c.exit(), "fx-btn fx-replay__btn fx-replay__exit");
    exit.setAttribute("aria-label", t("Salir de la repetición", "Exit replay"));
    this.wireScrub();
    const row = el("div", "fx-replay__row");
    row.append(back, this.play, fwd, ...this.speedBtns, this.drone, hide, exit);
    this.root.append(el("div", "fx-replay__tag", t("REPETICIÓN", "REPLAY")), this.scrub, this.time, row);
    this.root.hidden = true;
    this.hint.hidden = true;
    parent.append(this.root, this.hint);
    window.addEventListener("keydown", (e) => this.onKey(e));
  }

  get visible(): boolean { return this.shown; }

  show(on: boolean): void {
    this.shown = on;
    this.root.hidden = !on;
    this.lastScrub = this.lastSec = this.lastTotal = this.lastFlags = -1;
    if (!on) this.hint.hidden = true;
  }

  /** Pause menu open over the replay: ignore the replay keys. */
  setBlocked(on: boolean): void { this.blocked = on; }

  update(s: Readonly<ReplayStatus>): void {
    if (!this.shown) return;
    this.droneOn = s.drone;
    const pos = s.total > 0 ? Math.round((s.tick / s.total) * SCRUB_MAX) : 0;
    if (!this.dragging && pos !== this.lastScrub) { this.lastScrub = pos; this.scrub.value = String(pos); }
    const sec = Math.floor(s.tick / TICK_RATE);
    if (!this.dragging && (sec !== this.lastSec || s.total !== this.lastTotal)) {
      this.lastSec = sec; this.lastTotal = s.total;
      this.time.textContent = `${clock(s.tick)} / ${clock(s.total)}`;
    }
    const flags = (s.paused ? 1 : 0) | (s.drone ? 2 : 0) | (s.hudHidden ? 4 : 0) | (SPEEDS.indexOf(s.speed) << 3);
    if (flags !== this.lastFlags) this.renderFlags(s, flags);
  }

  private renderFlags(s: Readonly<ReplayStatus>, flags: number): void {
    const wasHidden = this.lastFlags >= 0 && (this.lastFlags & 4) !== 0;
    this.lastFlags = flags;
    this.play.textContent = s.paused ? "▶" : "⏸";
    this.play.setAttribute("aria-label", s.paused ? t("Reproducir", "Play") : t("Pausa", "Pause"));
    SPEEDS.forEach((v, i) => this.speedBtns[i].classList.toggle("is-on", v === s.speed));
    this.drone.classList.toggle("is-on", s.drone);
    this.root.hidden = s.hudHidden;
    if (s.hudHidden && !wasHidden) this.flashHint();
    if (!s.hudHidden) this.hint.hidden = true;
  }

  private flashHint(): void {
    this.hint.hidden = false;
    window.clearTimeout(this.hintTimer);
    this.hintTimer = window.setTimeout(() => { this.hint.hidden = true; }, HINT_MS);
  }

  /** Dragging previews the time; the seek (a re-simulation) happens on release. */
  private wireScrub(): void {
    const target = (): number => Number(this.scrub.value) / SCRUB_MAX;
    this.scrub.addEventListener("pointerdown", () => { this.dragging = true; });
    this.scrub.addEventListener("input", () => {
      this.dragging = true;
      if (this.lastTotal > 0) this.time.textContent = `${clock(target() * this.lastTotal)} / ${clock(this.lastTotal)}`;
    });
    this.scrub.addEventListener("change", () => {
      this.dragging = false;
      this.lastSec = -1;
      this.c.seekFraction(target());
    });
  }

  private onKey(e: KeyboardEvent): void {
    if (!this.shown || this.blocked || (e.repeat && e.code !== "ArrowLeft" && e.code !== "ArrowRight")) return;
    const tgt = e.target;
    if ((tgt instanceof HTMLInputElement && tgt !== this.scrub) || tgt instanceof HTMLTextAreaElement) return;
    const c = this.c;
    switch (e.code) {
      case "Space": if (this.droneOn) return; c.togglePause(); break;
      case "KeyP": c.togglePause(); break;
      case "ArrowLeft": c.skip(-SKIP_S); break;
      case "ArrowRight": c.skip(SKIP_S); break;
      case "Digit1": c.setSpeed(0.5); break;
      case "Digit2": c.setSpeed(1); break;
      case "Digit3": c.setSpeed(2); break;
      case "KeyH": c.toggleHud(); break;
      case "KeyC": c.toggleDrone(); break;
      default: return;
    }
    e.preventDefault();
  }
}
