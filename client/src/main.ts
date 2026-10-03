import { mountDevPanel, isDevMode } from "./dev/panel";
import { Engine, MatchSetup } from "./engine";
import { Hud } from "./hud";
import { Controls } from "./input";
import { GameLoop, LoopState } from "./loop";
import { CourtView } from "./render";
import { Difficulty, GameMode, MatchOptions, MenuUI, Settings } from "./ui/menu";

const RESET_KEY = "KeyR";
const RESPAWN_KEY = "KeyB";
const LOADING_TEXT = "Cargando…";
const MODE_ID: Readonly<Record<GameMode, MatchSetup["mode"]>> = { "1v1": 0, "2v2": 1, playground: 2 };
const DIFFICULTY_ID: Readonly<Record<Difficulty, number>> = { easy: 0, normal: 1, hard: 2, pro: 3 };
// Single local human on slot 0 in every mode: 1v1 rival, 2v2 partner + rivals are AI (filled inside wasm).
const HUMAN_MASK = 1;

function requireElement(id: string): HTMLElement {
  const e = document.getElementById(id);
  if (!e) throw new Error(`Falta el elemento #${id} en index.html`);
  return e;
}

const errText = (err: unknown): string => (err instanceof Error ? err.message : String(err));

function toSetup(o: MatchOptions): MatchSetup {
  return { mode: MODE_ID[o.mode], difficulty: DIFFICULTY_ID[o.difficulty], pointsToWin: o.pointsToWin, humanMask: HUMAN_MASK };
}

interface App {
  engine: Engine;
  view: CourtView;
  hud: Hud;
  controls: Controls;
  state: LoopState;
  mode: GameMode | null;
  pending: MatchOptions | null;
  refreshDev: (() => void) | null;
}

function startMatch(app: App, o: MatchOptions): void {
  if (!app.engine.loaded) {
    app.pending = o; // queued until wasm finishes loading
    app.hud.error(LOADING_TEXT);
    return;
  }
  app.hud.error("");
  app.engine.clearInputs();
  app.engine.configure(toSetup(o));
  app.engine.reset();
  app.mode = o.mode;
  app.state.running = true;
  app.state.paused = false;
  app.controls.releaseAll();
  app.controls.enabled = true;
  app.refreshDev?.();
}

function applySettings(app: App, s: Settings): void {
  app.view.applyVideo(s.video.quality, s.video.shadows);
  app.state.fpsCap = s.video.fpsCap;
  app.state.cameraShake = s.game.cameraShake;
  app.hud.setFpsVisible(s.video.showFps);
  app.hud.setScale(s.game.hudScale);
}

function wire(app: App, menu: MenuUI): void {
  menu.onStart((o) => {
    try { startMatch(app, o); } catch (err) { app.hud.error(errText(err)); }
  });
  menu.onPauseChange((paused) => {
    app.state.paused = paused;
    app.controls.enabled = !paused && app.state.running;
    app.controls.releaseAll();
  });
  menu.onQuit(() => {
    app.state.running = false;
    app.state.paused = false;
    app.controls.enabled = false;
    app.mode = null;
  });
  menu.onSettingsChange((s) => applySettings(app, s));
  applySettings(app, menu.settings);
  app.controls.onKeyPress((code) => {
    if (!app.state.running || app.state.paused) return;
    if (code === RESPAWN_KEY && app.mode === "playground") app.engine.respawnBall();
    if (code === RESET_KEY) app.engine.reset();
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
  if (queued) startMatch(app, queued);
}

function boot(): void {
  const hud = new Hud(requireElement("hud"));
  const menu = new MenuUI(requireElement("ui")); // menu first; wasm loads behind it
  const view = new CourtView(requireElement("app"));
  const controls = new Controls();
  controls.attach(view.canvas);
  const state: LoopState = { running: false, paused: false, fpsCap: 0, cameraShake: true };
  const app: App = { engine: new Engine(), view, hud, controls, state, mode: null, pending: null, refreshDev: null };
  wire(app, menu);
  new GameLoop(app.engine, view, hud, controls, state).start();
  void load(app);
}

try {
  boot();
} catch (err: unknown) {
  const hudRoot = document.getElementById("hud");
  if (hudRoot) hudRoot.textContent = `Error: ${errText(err)}`;
}
