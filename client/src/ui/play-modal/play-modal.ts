import { button, el } from "../dom";
import { CASUAL_CARDS, cardSection, modeCard, PlayPick, RANKED_CARDS, samePick } from "./cards";
import { historyPanel, infoPanel } from "./panels";
import { ProfileSummary } from "./side";

export type { PlayPick, QueueMode } from "./cards";

// "Elegí un modo" (docs/design/lobby.md): full-screen modal opened by JUGAR and the
// 1v1/2v2 pads. Single selection; JUGAR starts it. Esc / X close (a sub-sheet first).
// Built once; open() only toggles classes/text. Never touched per frame.
const SOON_MS = 1800;
const IDLE_LABEL = "ELEGÍ UN MODO";
const GO_LABEL = "JUGAR";

export class PlayModal {
  private readonly root = el("div", "fx-play", "", { role: "dialog", "aria-modal": "true", "aria-label": "Elegí un modo" });
  private readonly cards: Array<readonly [PlayPick, HTMLButtonElement]> = [];
  private readonly go = button(IDLE_LABEL, () => this.start(), "fx-play__go");
  private readonly sheet = el("section", "fx-play__sheet");
  private readonly toast = el("p", "fx-lobby__toast fx-play__toast", "Próximamente", { role: "status" });
  private readonly profile = new ProfileSummary();
  private readonly openCbs: Array<(open: boolean) => void> = [];
  private pick: PlayPick | null = null;
  private toastTimer = 0;

  constructor(parent: HTMLElement, private readonly onStart: (p: PlayPick) => void) {
    const mk = (defs: typeof CASUAL_CARDS): HTMLButtonElement[] => defs.map((d) => {
      const b = modeCard(d, (p) => this.select(p));
      this.cards.push([d, b]);
      return b;
    });
    const center = el("div", "fx-play__center");
    center.append(cardSection("Partidas casuales", "fx-play__section--casual", mk(CASUAL_CARDS)),
      cardSection("Clasificatorias", "fx-play__section--ranked", mk(RANKED_CARDS)));
    const left = el("div", "fx-play__left");
    left.append(this.profile.root);
    const right = el("nav", "fx-play__right", "", { "aria-label": "Más" });
    right.append(
      button("Historial", () => this.showSheet("Historial", historyPanel()), "fx-play__side"),
      button("Información", () => this.showSheet("Información", infoPanel()), "fx-play__side"),
      button("Recompensas", () => this.soon(), "fx-play__side"),
    );
    const foot = el("div", "fx-play__foot");
    foot.append(this.toast, this.go);
    const close = button("✕", () => this.close(), "fx-play__close");
    close.setAttribute("aria-label", "Cerrar");
    const grid = el("div", "fx-play__grid");
    grid.append(left, center, right);
    this.root.append(close, el("h2", "fx-play__title", "Elegí un modo"), grid, foot, this.sheet);
    this.root.hidden = true;
    this.sheet.hidden = true;
    this.toast.hidden = true;
    parent.append(this.root);
    window.addEventListener("keydown", (e) => this.onKey(e), true); // capture: beats MenuUI's Esc
  }

  get isOpen(): boolean { return !this.root.hidden; }
  onOpenChange(cb: (open: boolean) => void): void { this.openCbs.push(cb); }

  /** preselect: the pad's casual card; JUGAR opens with nothing selected. */
  open(preselect: PlayPick | null = null): void {
    if (this.isOpen) return;
    this.profile.refresh();
    this.sheet.hidden = true;
    this.select(preselect);
    this.root.hidden = false;
    this.emit(true);
    (this.cards.find(([p]) => samePick(this.pick, p))?.[1] ?? this.cards[0]?.[1])?.focus({ preventScroll: true });
  }

  close(): void {
    if (!this.isOpen) return;
    this.root.hidden = true;
    this.toast.hidden = true;
    window.clearTimeout(this.toastTimer);
    this.emit(false);
  }

  private select(p: PlayPick | null): void {
    this.pick = p;
    for (const [d, b] of this.cards) {
      const on = p !== null && samePick(p, d);
      b.classList.toggle("is-selected", on);
      b.setAttribute("aria-checked", String(on));
    }
    this.go.disabled = p === null;
    this.go.textContent = p === null ? IDLE_LABEL : GO_LABEL;
  }

  private start(): void {
    const p = this.pick;
    if (p === null) return;
    this.close();
    this.onStart(p);
  }

  private showSheet(title: string, body: HTMLElement): void {
    const back = button("Volver", () => { this.sheet.hidden = true; }, "fx-btn fx-btn--ghost");
    this.sheet.replaceChildren(el("h3", "fx-card__title", title), body, back);
    this.sheet.hidden = false;
    back.focus({ preventScroll: true });
  }

  private soon(): void {
    this.toast.hidden = false;
    window.clearTimeout(this.toastTimer);
    this.toastTimer = window.setTimeout(() => { this.toast.hidden = true; }, SOON_MS);
  }

  private emit(open: boolean): void { for (const cb of this.openCbs) cb(open); }

  private onKey(e: KeyboardEvent): void {
    if (!this.isOpen || e.code !== "Escape") return;
    e.preventDefault();
    e.stopImmediatePropagation();
    if (!this.sheet.hidden) this.sheet.hidden = true;
    else this.close();
  }
}
