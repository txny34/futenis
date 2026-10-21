import { Sound } from "./audio/sound";
import { Tutorial } from "./tutorial/tutorial";
import { setupAccount, type Account } from "./lobby/account";
import { extraOverlayOpen } from "./ui/overlays";
import { LANG, t } from "./i18n/t";
import { captureErrors } from "./report/errors";
import { buildReport } from "./report/report";
import { ChallengeSession } from "./profile/challenges";
import { applyThemeCss } from "./env/theme";
import { setHintKeys } from "./ui/key-hints";
import { mountDevPanel, isDevMode } from "./dev/panel";
import { Engine } from "./engine";
import { Hud } from "./hud";
import { Controls } from "./input";
import { GameLoop, LoopState } from "./loop";
import { BottomBar } from "./lobby/bottom-bar";
import { stylesOpener } from "./ui/styles/open-styles";
import { ProfileCard } from "./lobby/profile-card";
import { App, enterLobby, errText, landingAllowed, lobbyOptions, tutorialOptions, safeStart } from "./lobby/flow";
import { hookLoop } from "./loop-hooks";
import { startOnline } from "./lobby/online-flow";
import { isOnline } from "./lobby/online-state";
import { setupReplays, wireStoredReplays } from "./lobby/replay-player";
import { beginLocalTape } from "./lobby/replay-save";
import { CourtView } from "./render";
import { MatchScreens } from "./ui/match-screens";
import { MenuUI, Settings } from "./ui/menu";
import { PlayModal } from "./ui/play-modal/play-modal";

// Boot straight into the lobby (docs/design/lobby.md §7): no title, no main menu.
const RESET_KEY = "KeyR";
const FEED_KEY = "KeyB";

function requireElement(id: string): HTMLElement {
  const e = document.getElementById(id);
  if (!e) throw new Error(`${t("Falta el elemento", "Missing element")} #${id} ${t("en", "in")} index.html`);
  return e;
}

function applySettings(app: App, s: Settings): void {
  app.view.applyVideo(s.video.quality, s.video.shadows);
  app.state.fpsCap = s.video.fpsCap;
  app.state.cameraShake = s.game.cameraShake;
  app.state.showLanding = landingAllowed(s, app.last);
  app.view.rig.dynamic = s.game.dynamicCamera;
  app.view.rig.sensitivity = s.controls.mouseSensitivity;
  app.controls.setKeys(s.controls.keys);
  setHintKeys(s.controls.keys);
  app.hud.refreshKeys();
  app.hud.setFpsVisible(s.video.showFps);
  app.hud.setScale(s.game.hudScale);
  app.sound.setVolumes(s.audio);
}

/** Lobby overlays (Esc menu, "Elegí un modo"): controls off + mouse free while any is up. */
function lobbyOverlay(app: App): void {
  if (!app.inLobby) return; // in a match the pause callback owns the controls
  app.controls.enabled = !app.menu.isOpen && !app.play.isOpen && !extraOverlayOpen();
  app.controls.releaseAll();
}

function wire(app: App, menu: MenuUI): void {
  menu.onOpenChange(() => lobbyOverlay(app));
  app.play.onOpenChange(() => lobbyOverlay(app));
  menu.onPauseChange((paused) => {
    app.state.paused = paused && !isOnline(); // the server never pauses: keep stepping, send neutral input
    app.controls.enabled = !paused && app.state.running && !app.replaying; // the tape drives a replay
    app.replay.setBlocked(paused && app.replaying);
    app.controls.releaseAll();
  });
  menu.onQuit(() => {
    try { enterLobby(app); } catch (err) { app.hud.error(errText(err)); }
  });
  app.controls.onLook = (dx, dy) => app.view.rig.look(dx, dy);
  app.controls.onZoom = (d) => app.view.rig.zoom(d);
  app.controls.onLockChange = (locked) => app.hud.setCrosshair(locked);
  const openPauseOrLobbyMenu = (): void => {
    if (!app.state.running || menu.isOpen || app.play.isOpen || extraOverlayOpen()) return;
    if (app.inLobby) menu.openLobbyMenu();
    else menu.showPause();
  };
  // native Esc: the browser eats the keydown that drops pointer lock
  app.controls.onLockLost = openPauseOrLobbyMenu;
  app.controls.onMenu = openPauseOrLobbyMenu; // pad Start button: same as Esc
  menu.onSettingsChange((s) => applySettings(app, s));
  applySettings(app, menu.settings);
  app.controls.onKeyPress((code) => {
    if (!app.state.running || app.state.paused || app.replaying || isOnline()) return; // R/B would desync the tape / the server
    if (code === FEED_KEY && app.mode === "playground") app.engine.respawnBall();
    if (code === RESET_KEY && !app.inLobby) {
      app.engine.reset();
      if (app.last) beginLocalTape(app, app.last);
      app.loop?.rearm();
      app.hud.resetMatch();
      if (app.mode === "playground" && !app.last?.tutorial) app.challenges.begin(); // the sim's streak restarted too
    }
  });
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState !== "hidden" || !app.state.running || app.inLobby || app.state.paused) return;
    if (!menu.isOpen && !app.play.isOpen) menu.showPause();
  });
}

async function load(app: App): Promise<void> {
  try {
    await app.engine.load();
  } catch (err) {
    app.hud.error(`${t("No se pudo cargar el juego", "Could not load the game")}: ${errText(err)}`);
    return;
  }
  if (isDevMode()) app.refreshDev = mountDevPanel(app.engine, (m) => app.hud.error(m));
  const queued = app.pending;
  app.pending = null;
  try {
    if (queued) safeStart(app, queued);
    else enterLobby(app); // first visit: lobby + login + optional tutorial card (lobby/account.ts)
  } catch (err) {
    app.hud.error(errText(err));
  }
}

function lobbyBar(app: () => App): BottomBar {
  const styles = stylesOpener(() => app().menu.isOpen || app().play.isOpen || extraOverlayOpen(), () => lobbyOverlay(app()));
  return new BottomBar(document.body, { play: () => { if (!app().menu.isOpen) app().play.open(); }, styles });
}

function playModal(app: () => App): PlayModal {
  return new PlayModal(document.body, (p, kind) => {
    if (kind === "online") startOnline(app(), p.mode, p.ranked);
    else safeStart(app(), lobbyOptions(p.mode, false)); // bots: casual only, ELO is online ranked
  });
}

/** Esc in the lobby (lobby.md "Menú del lobby"): add future entries to `items`. */
function lobbyMenu(app: () => App, menu: MenuUI, account: Account): void {
  menu.setLobbyMenu(() => ({
    items: [
      { label: t("Tutorial", "Tutorial"), run: () => safeStart(app(), tutorialOptions()) },
      { label: t("Entrenamiento", "Training"), run: () => menu.openTraining((cfg) => safeStart(app(), lobbyOptions("playground", false, cfg))) },
      { label: t("Ajustes", "Settings"), run: () => menu.openSettings() },
      account.playersItem(),
      { label: t("Reportar un bug", "Report a bug"), run: () => menu.openReport() },
      account.menuItem(),
      ...account.accountItems(),
    ],
  }));
}

function boot(): void {
  captureErrors();
  document.documentElement.lang = LANG;
  applyThemeCss(document.documentElement); // --hud-* for overlays outside #hud (VS card, menus)
  const hudRoot = requireElement("hud");
  const hud = new Hud(hudRoot);
  const menu = new MenuUI(requireElement("ui"));
  const view = new CourtView(requireElement("app"));
  const controls = new Controls();
  controls.attach(view.canvas);
  const state: LoopState = { running: false, paused: false, fpsCap: 0, cameraShake: true, holdUntil: 0, showLanding: false, timeScale: 1 };
  const engine = new Engine();
  let ref: App | null = null;
  const get = (): App => { if (!ref) throw new Error(t("app no inicializada", "app not initialized")); return ref; };
  const app: App = {
    engine, view, hud, sound: new Sound(), tutorial: new Tutorial(requireElement("hud")), challenges: new ChallengeSession(), tape: null, replaying: false, replay: setupReplays(get, hudRoot), lastReplay: null, onlineLooks: null, onlineRanked: false, controls, state, bar: lobbyBar(get), profile: new ProfileCard(document.body), screens: new MatchScreens(document.body), menu, play: playModal(get),
    mode: null, inLobby: true, pending: null, refreshDev: null, loop: null, last: null,
  };
  ref = app;
  wireStoredReplays(app);
  const loop = new GameLoop(engine, view, hud, controls, state);
  hookLoop(app, loop);
  app.loop = loop;
  wire(app, menu);
  menu.setReport((note) => buildReport({
    note, snap: app.engine.loaded ? app.engine.peek().curr : null, inLobby: app.inLobby, settings: menu.settings,
  }));
  lobbyMenu(get, menu, setupAccount(app));
  enterLobby(app); // lobby decor + bar now; the sim spawns the player once wasm loads
  loop.start();
  void load(app);
}

try {
  boot();
} catch (err: unknown) {
  const hudRoot = document.getElementById("hud");
  if (hudRoot) hudRoot.textContent = `${t("Error", "Error")}: ${errText(err)}`;
}
