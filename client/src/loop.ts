import { Engine, MAX_STEPS, StepResult } from "./engine";
import { Hud } from "./hud";
import { Controls } from "./input";
import { CourtView, HumanView } from "./render";
import { EventKind, Quality, SimEvent } from "./snapshot";

const TICK_S = 1 / 120; // must match sim.TickRate
const MAX_FRAME_DT_S = MAX_STEPS * TICK_S; // one wasm call per frame (125 ms): beyond that the sim slows down
const MS_PER_S = 1000;
const FPS_SMOOTHING = 0.1;
const FPS_CAP_SLACK_MS = 0.5; // rAF jitter tolerance so a 60 cap on a 60 Hz screen never drops frames
export const HUMAN_SLOT = 0;

/** Mutable state the menu/settings drive. Read every frame, never reallocated. */
export interface LoopState {
  running: boolean;
  paused: boolean;
  fpsCap: number; // 0 = uncapped
  cameraShake: boolean;
}

/** Fixed-timestep loop: accumulate real time, step the sim at 1/120 s, render interpolated. */
export class GameLoop {
  private frame: StepResult;
  private acc = 0;
  private last = 0;
  private lastRender = 0;
  private fps = 0;
  private readonly human: HumanView = { slot: HUMAN_SLOT, targetX: 0, targetZ: 0, showOverlays: false };

  constructor(
    private readonly engine: Engine,
    private readonly view: CourtView,
    private readonly hud: Hud,
    private readonly controls: Controls,
    readonly state: LoopState,
  ) {
    this.frame = engine.peek();
  }

  start(): void {
    this.last = performance.now();
    this.lastRender = this.last;
    requestAnimationFrame(this.tick);
  }

  private readonly tick = (now: number): void => {
    requestAnimationFrame(this.tick);
    const cap = this.state.fpsCap;
    if (cap > 0 && now - this.lastRender < MS_PER_S / cap - FPS_CAP_SLACK_MS) return; // frame skip
    const dt = Math.min((now - this.last) / MS_PER_S, MAX_FRAME_DT_S);
    this.last = now;
    this.lastRender = now;
    if (dt > 0) this.fps += (1 / dt - this.fps) * FPS_SMOOTHING;
    try {
      this.simulate(dt, now);
    } catch (err) {
      this.state.running = false;
      this.hud.error(`Error de simulación: ${err instanceof Error ? err.message : String(err)}`);
    }
    const live = this.state.running && !this.state.paused;
    this.human.showOverlays = live;
    this.view.render(this.frame.prev, this.frame.curr, this.acc / TICK_S, live ? dt : 0, this.human);
    this.hud.update(this.frame.curr, now);
    this.hud.setFps(this.fps);
    this.hud.setSpin(this.controls.spinLevel);
  };

  private simulate(dt: number, now: number): void {
    const live = this.state.running && !this.state.paused && this.engine.loaded;
    if (!live) {
      this.acc = 0;
      return;
    }
    const input = this.controls.sample(this.view.camera);
    this.human.targetX = input.targetX;
    this.human.targetZ = input.targetZ;
    this.engine.setInput(HUMAN_SLOT, input);
    this.acc += dt;
    const n = Math.min(Math.floor(this.acc / TICK_S), MAX_STEPS);
    if (n > 0) {
      this.acc -= n * TICK_S;
      this.frame = this.engine.step(n);
      for (let i = 0; i < this.frame.eventCount; i++) this.onEvent(this.frame.events[i], now);
    }
    this.acc = Math.min(this.acc, TICK_S);
  }

  private onEvent(e: SimEvent, now: number): void {
    this.view.players.onEvent(e);
    this.hud.onEvent(e, HUMAN_SLOT, now);
    if (this.state.cameraShake && e.kind === EventKind.Kick && e.quality === Quality.Perfect) this.view.rig.shake();
  }
}
