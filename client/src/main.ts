import { Engine, StepResult } from "./engine";
import { Hud } from "./hud";
import { Keyboard } from "./input";
import { CourtView } from "./render";

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

/** Wires engine, input, view and HUD. No rules here. */
function startLoop(engine: Engine, view: CourtView, hud: Hud, keyboard: Keyboard): void {
  let frame: StepResult = engine.peek();
  let acc = 0;
  let last = performance.now();

  const loop = (now: number): void => {
    acc += Math.min((now - last) / MS_PER_S, MAX_FRAME_DT_S);
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

async function boot(): Promise<void> {
  const hud = new Hud(requireElement("hud"));
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
  keyboard.onKeyPress((code) => {
    if (code === RESET_KEY) engine.reset();
  });
  startLoop(engine, new CourtView(app), hud, keyboard);
}

boot().catch((err: unknown) => {
  const hudRoot = document.getElementById("hud");
  if (hudRoot) hudRoot.textContent = `Error: ${err instanceof Error ? err.message : String(err)}`;
});
