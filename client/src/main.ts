import { Engine, StepResult } from "./engine";
import { Hud } from "./hud";
import { Keyboard } from "./input";
import { CourtView } from "./render";
import { MatchOptions, MenuUI } from "./ui/menu";

const TICK_S = 1 / 60; // must match sim.TickRate
const MAX_FRAME_DT_S = 0.25;
const MAX_STEPS = 15;
const MS_PER_S = 1000;
const RESET_KEY = "KeyR";

function requireElement(id: string): HTMLElement {
  const e = document.getElementById(id);
  if (!e) throw new Error(`Falta el elemento #${id} en index.html`);
  return e;
}

/** Mutable loop gate driven by the menu: sim only steps while running and not paused. */
interface LoopGate { running: boolean; paused: boolean }

/** Wires engine, input, view and HUD. No rules here. */
function startLoop(engine: Engine, view: CourtView, hud: Hud, keyboard: Keyboard, gate: LoopGate): void {
  let frame: StepResult = engine.peek();
  let acc = 0;
  let last = performance.now();

  const loop = (now: number): void => {
    const live = gate.running && !gate.paused;
    acc = live ? acc + Math.min((now - last) / MS_PER_S, MAX_FRAME_DT_S) : 0;
    last = now;
    const n = Math.min(Math.floor(acc / TICK_S), MAX_STEPS);
    if (n > 0) {
      acc -= n * TICK_S;
      try {
        frame = engine.step(keyboard.action(0), keyboard.action(1), n);
      } catch (err) {
        hud.error(`Error de simulación: ${err instanceof Error ? err.message : String(err)}`);
        return;
      }
      keyboard.clearKicks();
    }
    acc = Math.min(acc, TICK_S);
    view.render(frame.prev, frame.curr, acc / TICK_S, keyboard.charge(0, now), keyboard.charge(1, now));
    hud.update(frame.curr);
    requestAnimationFrame(loop);
  };
  requestAnimationFrame(loop);
}

function wireMenu(menu: MenuUI, engine: Engine, gate: LoopGate): void {
  menu.onStart((opts: MatchOptions) => {
    // TODO(engine): pass opts.mode / opts.difficulty / opts.pointsToWin once the new wasm contract exposes them,
    // e.g. engine.configure(opts). For now every mode runs the existing 1v1 prototype.
    void opts;
    engine.reset();
    gate.running = true;
    gate.paused = false;
  });
  menu.onPauseChange((paused) => { gate.paused = paused; });
  menu.onQuit(() => { gate.running = false; gate.paused = false; });
  // TODO(render): apply menu.settings (quality, shadows, fpsCap, showFps, hudScale, cameraShake) via onSettingsChange.
}

async function boot(): Promise<void> {
  const hud = new Hud(requireElement("hud"));
  const menu = new MenuUI(requireElement("ui")); // menu first; wasm loads behind it
  const app = requireElement("app");
  const engine = new Engine();
  try {
    await engine.load();
  } catch (err) {
    hud.error(`No se pudo cargar el juego: ${err instanceof Error ? err.message : String(err)}`);
    return;
  }
  const keyboard = new Keyboard();
  keyboard.attach(window);
  const gate: LoopGate = { running: false, paused: false };
  keyboard.onKeyPress((code) => {
    if (code === RESET_KEY && gate.running && !gate.paused) engine.reset();
  });
  wireMenu(menu, engine, gate);
  startLoop(engine, new CourtView(app), hud, keyboard, gate);
}

boot().catch((err: unknown) => {
  const hudRoot = document.getElementById("hud");
  if (hudRoot) hudRoot.textContent = `Error: ${err instanceof Error ? err.message : String(err)}`;
});
