import { t } from "../i18n/t";
import { button, el } from "./dom";

// "Buscando rival…" while an online session waits: elapsed time, Cancelar and,
// for a private room, the shareable link with a copy button. The pointer stays
// free while it is up (the match grabs it on the first click on the court).
const TICK_MS = 1000;
const COPIED_MS = 1500;

export class SearchPanel {
  private readonly root = el("section", "fx-search", "", { role: "status", "aria-live": "polite" });
  private readonly title = el("b", "fx-search__title");
  private readonly clock = el("span", "fx-search__clock");
  private readonly share = el("div", "fx-search__share");
  private readonly link = el("code", "fx-search__link");
  private readonly copy = button(t("Copiar link", "Copy link"), () => this.copyLink(), "fx-btn fx-search__copy");
  private startedAt = 0;
  private timer = 0;

  constructor(parent: HTMLElement, onCancel: () => void) {
    this.share.append(el("span", "fx-search__hint", t("Pasale este link a tu amigo:", "Send this link to your friend:")), this.link, this.copy);
    this.root.append(this.title, this.clock, this.share, button(t("Cancelar", "Cancel"), onCancel, "fx-btn fx-btn--ghost fx-search__cancel"));
    this.root.hidden = true;
    parent.append(this.root);
  }

  get visible(): boolean { return !this.root.hidden; }

  /** link: private room URL to share, or "" for matchmaking. */
  show(title: string, link = ""): void {
    this.title.textContent = title;
    this.link.textContent = link;
    this.share.hidden = link === "";
    if (this.root.hidden) {
      this.startedAt = performance.now();
      this.render();
      window.clearInterval(this.timer);
      this.timer = window.setInterval(() => this.render(), TICK_MS);
    }
    this.root.hidden = false;
  }

  hide(): void {
    this.root.hidden = true;
    window.clearInterval(this.timer);
  }

  private render(): void {
    const s = Math.floor((performance.now() - this.startedAt) / 1000);
    this.clock.textContent = `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;
  }

  private copyLink(): void {
    const text = this.link.textContent ?? "";
    void navigator.clipboard?.writeText(text).then(() => {
      this.copy.textContent = t("¡Copiado!", "Copied!");
      window.setTimeout(() => { this.copy.textContent = t("Copiar link", "Copy link"); }, COPIED_MS);
    });
  }
}
