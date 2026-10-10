import { authUser, initAuth, logout } from "../auth/auth";
import { guestChosen } from "../auth/session";
import { t } from "../i18n/t";
import { LoginModal } from "../ui/login-modal";
import { errText, safeStart, tutorialOptions, type App } from "./flow";
import { TutorialPrompt } from "./tutorial-prompt";

// First visit = lobby + "Iniciá sesión" (Discord / Google / invitado) + an optional
// "¿Primera vez?" tutorial card. Ranked asks for an account again.

export interface Account {
  readonly login: LoginModal;
  /** Esc lobby menu entry: sign in, or sign out when signed in. */
  menuItem(): { label: string; run: () => void };
}

export function setupAccount(app: App): Account {
  const login = new LoginModal(document.body);
  new TutorialPrompt(app.bar.root, () => safeStart(app, tutorialOptions()));
  login.onOpenChange((open) => {
    if (!app.inLobby) return;
    app.controls.enabled = !open && !app.menu.isOpen && !app.play.isOpen;
    app.controls.releaseAll();
  });
  app.play.onNeedAccount(() => login.open(t("Las clasificatorias necesitan una cuenta.", "Ranked needs an account.")));
  void initAuth().then((err) => {
    if (err) app.hud.error(`${t("No se pudo iniciar sesión", "Could not sign in")}: ${err}`);
    if (!authUser() && (err || !guestChosen())) login.open();
  }).catch((err: unknown) => app.hud.error(errText(err)));
  return {
    login,
    menuItem: () => authUser()
      ? { label: t("Cerrar sesión", "Sign out"), run: () => { void logout(); } }
      : { label: t("Iniciar sesión", "Sign in"), run: () => login.open() },
  };
}
