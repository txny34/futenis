import { button, el } from "../dom";
import { BOT_CASUAL, CardDef, cardSection, modeCard, ONLINE_CASUAL, ONLINE_RANKED, PlayKind, PlayPick, samePick } from "./cards";
import { historyPanel, infoPanel } from "./panels";
import { ProfileSummary } from "./side";
import { t } from "../../i18n/t";
import { authUser } from "../../auth/auth";

export type { PlayKind, PlayPick, QueueMode } from "./cards";

// "Elegí un modo" (docs/design/lobby.md): full-screen modal opened by JUGAR and the
// 1v1/2v2 pads. Online first; the side "Bots" button swaps in the casual bot cards.
// Single selection; JUGAR starts it. Esc / X close (a sub-sheet first).
// Built once; open() only toggles classes/text. Never touched per frame.
const SOON_MS = 1800;
const IDLE_LABEL = t("ELEGÍ UN MODO", "PICK A MODE");
const GO_LABEL = t("JUGAR", "PLAY");
const SOON_LABEL = t("PRÓXIMAMENTE", "COMING SOON");

export class PlayModal {
  private readonly root = el("div", "fx-play", "", { role: "dialog", "aria-modal": "true", "aria-label": t("Elegí un modo", "Pick a mode") });
  private readonly cards: Array<readonly [CardDef, HTMLButtonElement, PlayKind]> = [];
  private readonly centers: Record<PlayKind, HTMLElement> = { online: el("div", "fx-play__center"), bots: el("div", "fx-play__center") };
  private readonly swap = button("", () => this.setKind(this.kind === "online" ? "bots" : "online"), "fx-play__side fx-play__side--swap");
  private kind: PlayKind = "online";
  private readonly go = button(IDLE_LABEL, () => this.start(), "fx-play__go");
  private readonly sheet = el("section", "fx-play__sheet");
  private readonly toast = el("p", "fx-lobby__toast fx-play__toast", t("Próximamente", "Coming soon"), { role: "status" });
  private readonly profile = new ProfileSummary();
  private readonly openCbs: Array<(open: boolean) => void> = [];
  private needAccount: () => void = () => undefined;
  private friend: () => void = () => undefined;
  private pick: PlayPick | null = null;
  private toastTimer = 0;

  constructor(parent: HTMLElement, private readonly onStart: (p: PlayPick, kind: PlayKind) => void) {
    const mk = (defs: readonly CardDef[], kind: PlayKind): HTMLButtonElement[] => defs.map((d) => {
      const b = modeCard(d, (p) => this.select(p));
      this.cards.push([d, b, kind]);
      return b;
    });
    this.centers.online.append(cardSection(t("Online · casuales", "Online · casual"), false, mk(ONLINE_CASUAL, "online")),
      cardSection(t("Clasificatorias", "Ranked"), true, mk(ONLINE_RANKED, "online")));
    this.centers.bots.append(cardSection(t("Contra bots · casuales", "Vs bots · casual"), false, mk(BOT_CASUAL, "bots")));
    const center = el("div", "fx-play__centers");
    center.append(this.centers.online, this.centers.bots);
    const left = el("div", "fx-play__left");
    left.append(this.profile.root);
    const right = el("nav", "fx-play__right", "", { "aria-label": t("Más", "More") });
    right.append(
      this.swap,
      button(t("Jugar con un amigo", "Play with a friend"), () => { this.close(); this.friend(); }, "fx-play__side"),
      button(t("Historial", "History"), () => this.showSheet(t("Historial", "History"), historyPanel()), "fx-play__side"),
      button(t("Información", "Info"), () => this.showSheet(t("Información", "Info"), infoPanel()), "fx-play__side"),
      button(t("Recompensas", "Rewards"), () => this.soon(), "fx-play__side"),
    );
    const foot = el("div", "fx-play__foot");
    foot.append(this.toast, this.go);
    const close = button("✕", () => this.close(), "fx-play__close");
    close.setAttribute("aria-label", t("Cerrar", "Close"));
    const grid = el("div", "fx-play__grid");
    grid.append(left, center, right);
    this.root.append(close, el("h2", "fx-play__title", t("Elegí un modo", "Pick a mode")), grid, foot, this.sheet);
    this.root.hidden = true;
    this.sheet.hidden = true;
    this.toast.hidden = true;
    parent.append(this.root);
    window.addEventListener("keydown", (e) => this.onKey(e), true); // capture: beats MenuUI's Esc
  }

  get isOpen(): boolean { return !this.root.hidden; }
  onOpenChange(cb: (open: boolean) => void): void { this.openCbs.push(cb); }
  /** Ranked picked without an account: the caller shows "Iniciá sesión". */
  onNeedAccount(cb: () => void): void { this.needAccount = cb; }
  /** "Jugar con un amigo": the caller opens a private room. */
  onFriend(cb: () => void): void { this.friend = cb; }

  /** preselect: the pad's card; JUGAR opens with nothing selected. Always opens online. */
  open(preselect: PlayPick | null = null): void {
    if (this.isOpen) return;
    this.profile.refresh();
    this.sheet.hidden = true;
    this.setKind("online");
    this.select(preselect);
    this.root.hidden = false;
    this.emit(true);
    const cards = this.visibleCards();
    (cards.find(([p]) => samePick(this.pick, p)) ?? cards[0])?.[1].focus({ preventScroll: true });
  }

  private visibleCards(): ReadonlyArray<readonly [CardDef, HTMLButtonElement, PlayKind]> {
    return this.cards.filter(([, , k]) => k === this.kind);
  }

  private setKind(kind: PlayKind): void {
    this.kind = kind;
    this.centers.online.hidden = kind !== "online";
    this.centers.bots.hidden = kind !== "bots";
    this.swap.textContent = kind === "online" ? t("Jugar contra bots", "Play vs bots") : t("Jugar online", "Play online");
    this.select(null);
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
    for (const [d, b, k] of this.cards) {
      const on = p !== null && k === this.kind && samePick(p, d);
      b.classList.toggle("is-selected", on);
      b.setAttribute("aria-checked", String(on));
    }
    const card = p === null ? undefined : this.visibleCards().find(([d]) => samePick(p, d))?.[0];
    this.go.disabled = p === null;
    this.go.textContent = p === null ? IDLE_LABEL : card?.available === false ? SOON_LABEL : GO_LABEL;
  }

  private start(): void {
    const p = this.pick;
    const card = this.visibleCards().find(([d]) => samePick(p, d))?.[0];
    if (p === null || !card) return;
    if (card.ranked && !authUser()) { this.close(); this.needAccount(); return; }
    if (!card.available) { this.soon(); return; }
    this.close();
    this.onStart(p, this.kind);
  }

  private showSheet(title: string, body: HTMLElement): void {
    const back = button(t("Volver", "Back"), () => { this.sheet.hidden = true; }, "fx-btn fx-btn--ghost");
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
