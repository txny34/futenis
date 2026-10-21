import { ReplayEngine } from "../replay/replay-engine";
import type { ReplayMeta } from "../replay/store";
import { decodeTape } from "../replay/tape-format";
import { ReplayBar, type ReplayControls, type ReplaySpeed, type ReplayStatus } from "../ui/replay-bar";
import { arm, enterLobby, errText, type App } from "./flow";
import { leaveOnline } from "./online-state";
import { openStoredReplay } from "./replay-flow";
import { endReplay } from "./replay-state";

// Plays a tape v2 (local or stored) on the wasm engine with the mini HUD:
// speed (loop timeScale), ±10 s / scrub (ReplayEngine.seek), drone camera, hide HUD.
const TICK_RATE = 120;
const HUD_OFF = "fx-hud-off";

export class ReplayPlayer implements ReplayControls {
  private engine: ReplayEngine | null = null;
  private bar: ReplayBar | null = null;
  private speed: ReplaySpeed = 1;
  private paused = false;
  private hudHidden = false;
  private readonly status: ReplayStatus = { tick: 0, total: 0, paused: false, speed: 1, drone: false, hudHidden: false };

  constructor(private readonly app: () => App, private readonly hudRoot: HTMLElement) {}

  /** Pause menu open/closed while replaying: the bar and the drone yield the input. */
  setBlocked(on: boolean): void {
    this.bar?.setBlocked(on);
    this.app().view.rig.setDroneBlocked(on);
  }

  /** Late-bound: the bar needs the player as its controls. */
  attach(bar: ReplayBar): void { this.bar = bar; }

  /** Plays raw tape bytes (local or stored). Throws on a bad tape (caller shows the error). */
  play(bytes: Uint8Array, meta: ReplayMeta | null): void {
    const tape = decodeTape(bytes);
    const app = this.app();
    endReplay(app);
    leaveOnline(app);
    const h = tape.header;
    const rewind = (): void => {
      app.engine.clearInputs();
      app.engine.configureFirst({ mode: h.mode === 1 ? 1 : 0, difficulty: h.difficulty, pointsToWin: h.pointsToWin, humanMask: h.initMask }, h.firstServer);
      app.engine.setConfig(tape.cfg);
      app.engine.reset();
      app.engine.setView(h.seat, h.mode === 1);
    };
    app.mode = h.mode === 1 ? "2v2" : "1v1";
    app.replaying = true;
    arm(app, false);
    rewind();
    app.controls.enabled = false; // the tape drives every slot; the camera follows the seat
    app.state.holdUntil = 0;
    this.engine = new ReplayEngine(app.engine, tape, rewind);
    app.loop?.setEngine(this.engine);
    app.hud.setReplay(true);
    app.view.players.setLooks(meta?.looks ?? null);
    this.speed = 1; this.paused = false; this.hudHidden = false;
    this.bar?.show(true);
    this.apply();
  }

  /** Back to normal play (endReplay): speed, camera, HUD and view restored. */
  stop(): void {
    const app = this.app();
    app.state.timeScale = 1;
    app.view.rig.setDrone(false);
    this.setBlocked(false);
    this.bar?.show(false);
    this.hudRoot.classList.remove(HUD_OFF);
    app.engine.setView(0, false);
    app.view.players.setLooks(null);
    this.engine = null;
  }

  /** Per frame (loop.onFrame). No allocations. */
  frame(): void {
    const e = this.engine;
    if (!e) return;
    if (e.ended && !this.paused) this.onEnded(); // a tape that stops before GameOver
    const s = this.status;
    s.tick = e.tick; s.total = e.total; s.speed = this.speed; s.paused = this.paused;
    s.drone = this.app().view.rig.droneOn; s.hudHidden = this.hudHidden;
    this.bar?.update(s);
  }

  /** Match over during a replay: freeze at the end, keep the bar. */
  onEnded(): void { this.paused = true; this.apply(); }

  togglePause(): void {
    const e = this.engine;
    if (!e) return;
    if (this.paused && e.ended) { this.seekTo(0); return; } // play again from the start
    this.paused = !this.paused;
    this.apply();
  }

  setSpeed(s: ReplaySpeed): void { this.speed = s; this.apply(); }
  skip(sec: number): void { if (this.engine) this.seekTo(this.engine.tick + sec * TICK_RATE); }
  seekFraction(f: number): void { if (this.engine) this.seekTo(f * this.engine.total); }
  toggleDrone(): void { const rig = this.app().view.rig; rig.setDrone(!rig.droneOn); }
  toggleHud(): void { this.hudHidden = !this.hudHidden; this.apply(); }

  exit(): void {
    const app = this.app();
    try { enterLobby(app); } catch (err) { app.hud.error(errText(err)); }
  }

  private seekTo(target: number): void {
    const e = this.engine;
    if (!e) return;
    const wasEnded = e.ended;
    e.seek(target);
    this.app().loop?.rearm(); // a later match over fires again; reactions reset
    if (wasEnded && e.tick < e.total) this.paused = false;
    this.apply();
  }

  private apply(): void {
    this.app().state.timeScale = this.paused ? 0 : this.speed;
    this.hudRoot.classList.toggle(HUD_OFF, this.hudHidden);
  }
}

/** Builds the player + bar and wires Historial's ▶. Returns the player for App.replay. */
export function setupReplays(app: () => App, hudRoot: HTMLElement): ReplayPlayer {
  const player = new ReplayPlayer(app, hudRoot);
  player.attach(new ReplayBar(document.body, player));
  return player;
}

/** Historial ▶ → play that stored replay. Call once the app exists. */
export function wireStoredReplays(app: App): void {
  app.play.onReplay((id) => { void openStoredReplay(app, id); });
}
