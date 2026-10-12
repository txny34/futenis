import { authUser, initAuth, logout, onAuthChange } from "../auth/auth";
import { syncRatings } from "../auth/ratings";
import { guestChosen } from "../auth/session";
import { t } from "../i18n/t";
import { startCloudSync } from "../profile/cloud";
import { profileWriter } from "../profile/store";
import { LoginModal } from "../ui/login-modal";
import { lobbyPlayersView } from "../ui/lobby-players";
import { extraOverlayOpen, registerOverlay } from "../ui/overlays";
import { errText, safeStart, tutorialOptions, type App } from "./flow";
import { TutorialPrompt } from "./tutorial-prompt";
import { startPresence } from "./presence";
import { bindEmotes } from "./emotes";
import { joinFromUrl, retryLinkJoin, startFriendRoom } from "./online-flow";

// First visit = lobby + "Iniciá sesión" (Discord / Google / invitado) + an optional
// "¿Primera vez?" tutorial card. Ranked asks for an account again.

export interface Account {
  readonly login: LoginModal;
  /** Esc lobby menu entry: sign in, or sign out when signed in. */
  menuItem(): { label: string; run: () => void };
  /** Esc lobby menu entry: who is in the shared lobby (hide / report). */
  playersItem(): { label: string; run: () => void };
}

const SIGN_IN_FAILED = t("No se pudo iniciar sesión. Probá de nuevo.", "Could not sign in. Please try again.");

export function setupAccount(app: App): Account {
  const login = new LoginModal(document.body);
  registerOverlay(() => login.isOpen);
  new TutorialPrompt(app.bar.root, () => safeStart(app, tutorialOptions()));
  const presence = startPresence(app, app.view.lobby.remote); // shared online lobby
  bindEmotes(app, presence); // 1-4: saludo, festejo, baile, bajón
  const remotes = app.view.lobby.remote;
  app.play.onFriend(() => startFriendRoom(app));
  joinFromUrl(app); // opened from a friend's link (?room=CODE)
  login.onOpenChange((open) => { // also mid-session (a friend's link opened behind the login)
    app.controls.enabled = !app.menu.isOpen && !app.play.isOpen && !extraOverlayOpen() && !app.state.paused && !app.replaying;
    app.controls.releaseAll();
    if (!open) retryLinkJoin();
  });
  app.play.onNeedAccount(() => login.open(t("Las clasificatorias necesitan una cuenta.", "Ranked needs an account.")));
  onAuthChange((u) => {
    profileWriter().update((p) => ({ ...p })); // re-render the shown name
    if (u) void syncRatings(u.id); // ranked ELO comes from the server
  });
  startCloudSync(); // level, XP, missions and history follow the account
  void initAuth().then((err) => {
    if (err) login.open(SIGN_IN_FAILED); // in the modal: survives the lobby clearing the HUD
    else if (!authUser() && !guestChosen()) login.open("", true);
  }).catch((err: unknown) => app.hud.error(errText(err)));
  return {
    login,
    playersItem: () => ({
      label: t("Jugadores en el lobby", "Players in the lobby"),
      run: () => app.menu.openPanel((back) => lobbyPlayersView({
        list: () => remotes.list(), setHidden: (id, on) => remotes.setHidden(id, on), report: (id) => presence.report(id),
      }, back)),
    }),
    menuItem: () => authUser()
      ? { label: t("Cerrar sesión", "Sign out"), run: () => { app.menu.hide(); void logout().then(() => login.open("", true)); } }
      : { label: t("Iniciar sesión", "Sign in"), run: () => { app.menu.hide(); login.open(); } },
  };
}
