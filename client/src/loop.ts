import { HumanInput, MAX_STEPS, StepEngine, StepResult } from "./engine";
import { Hype, HypeMoment, LONG_RALLY } from "./feel/hype";
import { MatchResult, MatchTally } from "./feel/tally";
import { ShotFeed } from "./feel/shots";
import { Hud } from "./hud";
import { t } from "./i18n/t";
import { Controls } from "./input";
import { CourtView, HumanView, ScreenPoint } from "./render";
import { EventKind, Mode, Phase, Quality, SimEvent, Snap } from "./snapshot";

const TICK_S = 1 / 120; // must match sim.TickRate
const MAX_FRAME_DT_S = MAX_STEPS * TICK_S; // one wasm call per frame (125 ms): beyond that the sim slows down
const MS_PER_S = 1000;
const FPS_SMOOTHING = 0.1;
const FPS_CAP_SLACK_MS = 0.5; // rAF jitter tolerance so a 60 cap on a 60 Hz screen never drops frames
const HIT_STOP_MS = 50; // perfect-hit freeze frame
const POPUP_LIFT_M = 0.8; // popup starts this far above the contact point
const POPUP_DIR_SPEED = 4; // m/s of lateral (Z) shot speed mapped to full drift
const SERVE_AIM_MIN_X = 1; // serve aim assist: keep the crosshair target inside the service box depth
const SERVE_AIM_MAX_X = 6.2; // a bit short of the service line (6.95)
const RALLY_CHEER = 0.5; // live rally excitement tops out at half a full cheer
export const HUMAN_SLOT = 0;
const HUMAN_TEAM = 0; // slot 0 plays for team 0 (online guests get a rotated view)

/** Mutable state the menu/settings drive. Read every frame, never reallocated. */
export interface LoopState {
  running: boolean;
  paused: boolean;
  fpsCap: number; // 0 = uncapped
  cameraShake: boolean;
  /** performance.now() ms until which the sim is held (VS intro). */
  holdUntil: number;
  /** Landing marker allowed for the current match (settings x difficulty). */
  showLanding: boolean;
}

export type { MatchResult } from "./feel/tally";

/** Fixed-timestep loop: accumulate real time, step the sim at 1/120 s, render interpolated. */
export class GameLoop {
  /** Fires once when a scored match reaches Phase.GameOver; call rearm() on a new match. */
  onMatchOver: ((r: Readonly<MatchResult>, snap: Float64Array) => void) | null = null;
  /** Lobby pad events (EvPadEnter/Leave/Start, lobby.md §4). Fired after the step's other events. */
  onPadEvent: ((kind: number, pad: number) => void) | null = null;
  /** A point that closed a long rally or extended a streak (crowd roar). */
  onHype: ((m: Readonly<HypeMoment>) => void) | null = null;
  /** After each step call: tick count + the input sent (replay tape). */
  onStep: ((n: number, input: Readonly<HumanInput>) => void) | null = null;
  /** Every non-lobby sim event (profile/recorder.ts: perfects, training streak). */
  onGameEvent: ((e: SimEvent) => void) | null = null;
  /** After each render: the current snapshot and the live dt (0 while paused or frozen). */
  onFrame: ((snap: Float64Array, dt: number) => void) | null = null;
  private frame: StepResult;
  private acc = 0;
  private last = 0;
  private lastRender = 0;
  private fps = 0;
  private hitStopUntil = 0;
  private overFired = false;
  private stepped = false;
  private readonly tally = new MatchTally();
  private readonly shots = new ShotFeed();
  private readonly hype = new Hype();
  private readonly point: ScreenPoint = { x: 0, y: 0 };
  private readonly human: HumanView = { slot: HUMAN_SLOT, targetX: 0, targetZ: 0, showOverlays: false, showLanding: false };

  constructor(
    private engine: StepEngine,
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

  /** Swaps the simulation source (live wasm <-> replay). */
  setEngine(e: StepEngine): void {
    this.engine = e;
    this.frame = e.peek();
  }

  rearm(): void {
    this.overFired = false;
    this.stepped = false; // the engine's snapshot still shows the previous match until the next step
    this.shots.reset();
    this.hype.reset();
    this.tally.reset();
    this.view.players.resetReactions(); // no festejo/bajón carried into the next match
  }

  private readonly tick = (now: number): void => {
    requestAnimationFrame(this.tick);
    const cap = this.state.fpsCap;
    if (cap > 0 && now - this.lastRender < MS_PER_S / cap - FPS_CAP_SLACK_MS) return; // frame skip
    const dt = Math.min((now - this.last) / MS_PER_S, MAX_FRAME_DT_S);
    this.last = now;
    this.lastRender = now;
    if (dt > 0) this.fps += (1 / dt - this.fps) * FPS_SMOOTHING;
    const frozen = now < this.hitStopUntil;
    try {
      if (!frozen) this.simulate(dt, now);
    } catch (err) {
      this.state.running = false;
      const msg = err instanceof Error ? err.message : String(err);
      this.hud.error(t(`Error de simulación: ${msg}`, `Simulation error: ${msg}`));
    }
    const live = this.state.running && !this.state.paused;
    this.human.showOverlays = live;
    this.human.showLanding = this.state.showLanding;
    this.view.render(this.frame.prev, this.frame.curr, this.acc / TICK_S, live && !frozen ? dt : 0, this.human);
    this.onFrame?.(this.frame.curr, live && !frozen ? dt : 0);
    this.hud.update(this.frame.curr, now);
    this.hud.setFps(this.fps);
    this.checkMatchOver();
  };

  private simulate(dt: number, now: number): void {
    const live = this.state.running && !this.state.paused && this.engine.loaded && now >= this.state.holdUntil;
    if (!live) {
      this.acc = 0;
      return;
    }
    const input = this.controls.sample(this.view.camera, dt);
    this.assistServeAim(input);
    this.human.targetX = input.targetX;
    this.human.targetZ = input.targetZ;
    this.engine.setInput(HUMAN_SLOT, input);
    this.acc += dt;
    const n = Math.min(Math.floor(this.acc / TICK_S), MAX_STEPS);
    if (n > 0) {
      this.acc -= n * TICK_S;
      this.frame = this.engine.step(n);
      this.onStep?.(n, input);
      this.stepped = true;
      for (let i = 0; i < this.frame.eventCount; i++) this.onEvent(this.frame.events[i], now);
    }
    this.acc = Math.min(this.acc, TICK_S);
  }

  /** While the human serves, the aim depth is held inside the service box (the box is drawn on court). */
  private assistServeAim(input: { targetX: number }): void {
    const s = this.frame.curr;
    if (s[Snap.Phase] !== Phase.Serve || s[Snap.Server] !== 0) return;
    input.targetX = Math.max(SERVE_AIM_MIN_X, Math.min(SERVE_AIM_MAX_X, input.targetX));
  }

  private onEvent(e: SimEvent, now: number): void {
    if (e.kind >= EventKind.PadEnter && e.kind <= EventKind.PadStart) {
      this.onPadEvent?.(e.kind, e.side);
      return;
    }
    if (e.kind === EventKind.BounceFloor && this.shots.rematePending) this.view.fx.splat(e.x, e.z);
    this.onGameEvent?.(e);
    const shot = this.shots.onEvent(e);
    this.view.players.onEvent(e);
    this.hud.onEvent(e, shot, now);
    this.feelHype(e, now);
    if (e.kind === EventKind.Feed) this.view.machine.flash();
    if (e.kind === EventKind.Kick) this.onKick(e, now);
    else this.tally.onEvent(e);
  }

  private feelHype(e: SimEvent, now: number): void {
    if (this.frame.curr[Snap.Mode] === Mode.Playground) return; // no crowd moments vs the ball machine
    const m = this.hype.onEvent(e);
    if (e.kind === EventKind.Kick) this.view.cheer(this.hype.rally01 * RALLY_CHEER);
    if (!m || (m.touches < LONG_RALLY && m.winner !== HUMAN_TEAM)) return; // rival streaks stay quiet
    this.view.cheer(1);
    this.hud.moment(m, now);
    this.onHype?.(m);
  }

  private onKick(e: SimEvent, now: number): void {
    const perfect = e.quality === Quality.Perfect;
    this.view.fx.impact(e.x, e.y, e.z, e.quality);
    if (perfect) this.view.players.flash(e.player);
    if (e.player !== HUMAN_SLOT) return;
    const p = this.view.screenOf(e.x, e.y + POPUP_LIFT_M, e.z, this.point);
    this.hud.hitPopup(e.quality, p.x, p.y, e.vz / POPUP_DIR_SPEED, now);
    if (!perfect) return;
    this.hitStopUntil = now + HIT_STOP_MS;
    if (this.state.cameraShake) this.view.rig.shake();
  }

  private checkMatchOver(): void {
    const c = this.frame.curr;
    if (this.overFired || !this.stepped || !this.state.running || c[Snap.Phase] !== Phase.GameOver || c[Snap.Mode] === Mode.Playground) return;
    this.overFired = true;
    this.onMatchOver?.(this.tally.result, c);
  }
}
