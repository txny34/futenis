import { authUser, initAuth, logout, onAuthChange } from "../auth/auth";
import { syncRatings } from "../auth/ratings";
import { guestChosen } from "../auth/session";
import { t } from "../i18n/t";
import { profileWriter } from "../profile/store";
import { LoginModal } from "../ui/login-modal";
import { extraOverlayOpen, registerOverlay } from "../ui/overlays";
import { errText, safeStart, tutorialOptions, type App } from "./flow";
import { TutorialPrompt } from "./tutorial-prompt";
import { startPresence } from "./presence";

// First visit = lobby + "Iniciá sesión" (Discord / Google / invitado) + an optional
// "¿Primera vez?" tutorial card. Ranked asks for an account again.

export interface Account {
  readonly login: LoginModal;
  /** Esc lobby menu entry: sign in, or sign out when signed in. */
  menuItem(): { label: string; run: () => void };
}

const SIGN_IN_FAILED = t("No se pudo iniciar sesión. Probá de nuevo.", "Could not sign in. Please try again.");

export function setupAccount(app: App): Account {
  const login = new LoginModal(document.body);
  registerOverlay(() => login.isOpen);
  new TutorialPrompt(app.bar.root, () => safeStart(app, tutorialOptions()));
  startPresence(app, app.view.lobby.remote); // shared online lobby
  login.onOpenChange(() => {
    if (!app.inLobby) return;
    app.controls.enabled = !app.menu.isOpen && !app.play.isOpen && !extraOverlayOpen();
    app.controls.releaseAll();
  });
  app.play.onNeedAccount(() => login.open(t("Las clasificatorias necesitan una cuenta.", "Ranked needs an account.")));
  onAuthChange((u) => {
    profileWriter().update((p) => ({ ...p })); // re-render the shown name
    if (u) void syncRatings(u.id); // ranked ELO comes from the server
  });
  void initAuth().then((err) => {
    if (err) login.open(SIGN_IN_FAILED); // in the modal: survives the lobby clearing the HUD
    else if (!authUser() && !guestChosen()) login.open("", true);
  }).catch((err: unknown) => app.hud.error(errText(err)));
  return {
    login,
    menuItem: () => authUser()
      ? { label: t("Cerrar sesión", "Sign out"), run: () => { app.menu.hide(); void logout().then(() => login.open("", true)); } }
      : { label: t("Iniciar sesión", "Sign in"), run: () => { app.menu.hide(); login.open(); } },
  };
}
