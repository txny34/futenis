import type { SimEvent } from "./snapshot";
import { Sound } from "./audio/sound";
import { Tutorial, tutorialDone } from "./tutorial/tutorial";
import { captureErrors } from "./report/errors";
import { buildReport } from "./report/report";
import { LONG_RALLY } from "./feel/hype";
import { ChallengeSession, MEDALS } from "./profile/challenges";
import { applyThemeCss } from "./env/theme";
import { setHintKeys } from "./ui/key-hints";
import { mountDevPanel, isDevMode } from "./dev/panel";
import { Engine } from "./engine";
import { Hud } from "./hud";
import { Controls } from "./input";
import { GameLoop, LoopState } from "./loop";
import { BottomBar } from "./lobby/bottom-bar";
import { ProfileCard } from "./lobby/profile-card";
import { recorder } from "./profile/recorder";
import { App, enterLobby, errText, landingAllowed, lobbyOptions, tutorialOptions, onPadEvent, safeStart } from "./lobby/flow";
import { showResult } from "./lobby/replay-flow";
import { Tape } from "./replay/tape";
import { CourtView } from "./render";
import { MatchScreens } from "./ui/match-screens";
import { MenuUI, Settings } from "./ui/menu";
import { PlayModal } from "./ui/play-modal/play-modal";

// Boot straight into the lobby (docs/design/lobby.md §7): no title, no main menu.
const RESET_KEY = "KeyR";
const FEED_KEY = "KeyB";
const HYPE_STREAK_ROAR = 0.6; // a streak gets a smaller roar than a puntazo
const MEDAL_ROAR = 0.5;

function requireElement(id: string): HTMLElement {
  const e = document.getElementById(id);
  if (!e) throw new Error(`Falta el elemento #${id} en index.html`);
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
  app.controls.enabled = !app.menu.isOpen && !app.play.isOpen;
  app.controls.releaseAll();
}

function wire(app: App, menu: MenuUI): void {
  menu.onOpenChange(() => lobbyOverlay(app));
  app.play.onOpenChange(() => lobbyOverlay(app));
  menu.onPauseChange((paused) => {
    app.state.paused = paused;
    app.controls.enabled = !paused && app.state.running && !app.replaying; // the tape drives a replay
    app.controls.releaseAll();
  });
  menu.onQuit(() => {
    try { enterLobby(app); } catch (err) { app.hud.error(errText(err)); }
  });
  app.controls.onLook = (dx, dy) => app.view.rig.look(dx, dy);
  app.controls.onZoom = (d) => app.view.rig.zoom(d);
  app.controls.onLockChange = (locked) => app.hud.setCrosshair(locked);
  const openPauseOrLobbyMenu = (): void => {
    if (!app.state.running || menu.isOpen || app.play.isOpen) return;
    if (app.inLobby) menu.openLobbyMenu();
    else menu.showPause();
  };
  // native Esc: the browser eats the keydown that drops pointer lock
  app.controls.onLockLost = openPauseOrLobbyMenu;
  app.controls.onMenu = openPauseOrLobbyMenu; // pad Start button: same as Esc
  menu.onSettingsChange((s) => applySettings(app, s));
  applySettings(app, menu.settings);
  app.controls.onKeyPress((code) => {
    if (!app.state.running || app.state.paused || app.replaying) return; // R/B would desync the tape
    if (code === FEED_KEY && app.mode === "playground") app.engine.respawnBall();
    if (code === RESET_KEY && !app.inLobby) {
      app.engine.reset();
      app.tape.clear(app.engine.getConfig());
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
    app.hud.error(`No se pudo cargar el juego: ${errText(err)}`);
    return;
  }
  if (isDevMode()) app.refreshDev = mountDevPanel(app.engine, (m) => app.hud.error(m));
  const queued = app.pending;
  app.pending = null;
  try {
    if (queued) safeStart(app, queued);
    else if (!tutorialDone()) safeStart(app, tutorialOptions()); // first visit: learn before the lobby
    else enterLobby(app);
  } catch (err) {
    app.hud.error(errText(err));
  }
}

function lobbyBar(app: () => App): BottomBar {
  return new BottomBar(document.body, { play: () => { if (!app().menu.isOpen) app().play.open(); } });
}

function playModal(app: () => App): PlayModal {
  return new PlayModal(document.body, (p) => safeStart(app(), lobbyOptions(p.mode, p.ranked)));
}

/** Esc in the lobby (lobby.md "Menú del lobby"): add future entries to `items`. */
function lobbyMenu(app: () => App, menu: MenuUI): void {
  menu.setLobbyMenu(() => ({
    items: [
      { label: "Tutorial", run: () => safeStart(app(), tutorialOptions()) },
      { label: "Entrenamiento", run: () => menu.openTraining((t) => safeStart(app(), lobbyOptions("playground", false, t))) },
      { label: "Ajustes", run: () => menu.openSettings() },
      { label: "Reportar un bug", run: () => menu.openReport() },
    ],
  }));
}

function onMedal(app: App, e: SimEvent): void {
  const earned = app.challenges.onEvent(e);
  if (earned.length === 0) return;
  const top = earned[earned.length - 1]; // banner shows one; every medal is saved
  app.hud.medal(`¡${MEDALS[top.medal].toUpperCase()}! ${top.def.title}`, performance.now());
  app.sound.cheer(MEDAL_ROAR);
}

function boot(): void {
  captureErrors();
  applyThemeCss(document.documentElement); // --hud-* for overlays outside #hud (VS card, menus)
  const hud = new Hud(requireElement("hud"));
  const menu = new MenuUI(requireElement("ui"));
  const view = new CourtView(requireElement("app"));
  const controls = new Controls();
  controls.attach(view.canvas);
  const state: LoopState = { running: false, paused: false, fpsCap: 0, cameraShake: true, holdUntil: 0, showLanding: false };
  const engine = new Engine();
  let ref: App | null = null;
  const get = (): App => { if (!ref) throw new Error("app no inicializada"); return ref; };
  const app: App = {
    engine, view, hud, sound: new Sound(), tutorial: new Tutorial(requireElement("hud")), challenges: new ChallengeSession(), tape: new Tape(), replaying: false, controls, state, bar: lobbyBar(get), profile: new ProfileCard(document.body), screens: new MatchScreens(document.body), menu, play: playModal(get),
    mode: null, inLobby: true, pending: null, refreshDev: null, loop: null, last: null,
  };
  ref = app;
  const loop = new GameLoop(engine, view, hud, controls, state);
  loop.onMatchOver = (r, snap) => showResult(app, r, snap);
  loop.onStep = (n, a) => { if (!app.replaying && !app.inLobby && app.mode !== "playground") app.tape.push(n, a); };
  loop.onGameEvent = (e) => { recorder().onEvent(e); app.sound.onEvent(e); app.tutorial.onEvent(e); onMedal(app, e); };
  loop.onHype = (m) => app.sound.cheer(m.touches >= LONG_RALLY ? 1 : HYPE_STREAK_ROAR);
  loop.onPadEvent = (kind, pad) => onPadEvent(app, kind, pad);
  app.loop = loop;
  wire(app, menu);
  menu.setReport((note) => buildReport({
    note, snap: app.engine.loaded ? app.engine.peek().curr : null, inLobby: app.inLobby, settings: menu.settings,
  }));
  lobbyMenu(get, menu);
  enterLobby(app); // lobby decor + bar now; the sim spawns the player once wasm loads
  loop.start();
  void load(app);
}

try {
  boot();
} catch (err: unknown) {
  const hudRoot = document.getElementById("hud");
  if (hudRoot) hudRoot.textContent = `Error: ${errText(err)}`;
}
