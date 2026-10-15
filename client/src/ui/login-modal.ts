import type { Provider } from "../auth/config";
import { chooseGuest, signIn } from "../auth/session";
import { t } from "../i18n/t";
import { button, el } from "./dom";

function legalLine(): HTMLElement {
  const link = (href: string, text: string): HTMLAnchorElement =>
    el("a", "", text, { href, target: "_blank", rel: "noopener noreferrer" });
  const p = el("p", "fx-login__note fx-login__legal", t("Al jugar aceptás los ", "By playing you accept the "));
  p.append(link("terms.html", t("Términos", "Terms")), t(" y la ", " and the "), link("privacy.html", t("Política de privacidad", "Privacy policy")), ".");
  return p;
}

// "Iniciá sesión": Discord / Google / Jugar como invitado. Shown in the lobby on
// a first visit and whenever a guest picks something that needs an account.
export class LoginModal {
  private readonly root = el("div", "fx-login", "", { role: "dialog", "aria-modal": "true", "aria-labelledby": "fx-login-title", "aria-describedby": "fx-login-reason" });
  private readonly reason = el("p", "fx-login__reason", "", { id: "fx-login-reason", role: "status" });
  private firstVisit = false;
  private leaving = false;
  private readonly openCbs: Array<(open: boolean) => void> = [];

  constructor(parent: HTMLElement) {
    const card = el("section", "fx-card fx-login__card");
    const providers = el("div", "fx-stack");
    providers.append(
      this.provider("discord", t("Entrar con Discord", "Sign in with Discord")),
      this.provider("google", t("Entrar con Google", "Sign in with Google")),
    );
    const guest = button(t("Jugar como invitado", "Play as guest"), () => { chooseGuest(); this.close(); }, "fx-btn fx-btn--ghost fx-login__guest");
    card.append(el("h2", "fx-card__title", t("Iniciá sesión", "Sign in"), { id: "fx-login-title" }), this.reason, providers,
      el("p", "fx-login__note", t("Con cuenta: clasificatorias con ELO y tu progreso en cualquier dispositivo.", "With an account: ranked with ELO and your progress on any device.")),
      guest, legalLine());
    this.root.append(card);
    this.root.hidden = true;
    parent.append(this.root);
    window.addEventListener("keydown", (e) => this.onKey(e), true);
  }

  get isOpen(): boolean { return !this.root.hidden; }
  onOpenChange(cb: (open: boolean) => void): void { this.openCbs.push(cb); }

  /** reason: why we ask now (ranked, a failed sign-in); firstVisit: Esc means "guest". */
  open(reason = "", firstVisit = false): void {
    this.firstVisit = firstVisit;
    this.reason.textContent = reason;
    this.reason.hidden = reason === "";
    if (this.isOpen) return;
    this.root.hidden = false;
    for (const cb of this.openCbs) cb(true);
    this.root.querySelector("button")?.focus({ preventScroll: true });
  }

  close(): void {
    if (!this.isOpen) return;
    this.root.hidden = true;
    for (const cb of this.openCbs) cb(false);
  }

  private provider(p: Provider, label: string): HTMLButtonElement {
    return button(label, () => {
      if (this.leaving) return; // double click: one redirect
      this.leaving = true;
      void signIn(p).finally(() => { this.leaving = false; });
    }, `fx-btn fx-login__btn fx-login__btn--${p}`);
  }

  private onKey(e: KeyboardEvent): void {
    if (!this.isOpen || e.code !== "Escape") return;
    e.preventDefault();
    e.stopImmediatePropagation();
    if (this.firstVisit) chooseGuest(); // Esc on the first-visit prompt = play as a guest
    this.close();
  }
}
