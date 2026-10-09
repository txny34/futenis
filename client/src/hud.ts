import { applyThemeCss } from "./env/theme";
import type { ShotKindId } from "./feel/shots";
import { ActionBar } from "./hud-actions";
import { Callouts } from "./hud-callouts";
import { CallTracker } from "./hud-calls";
import { Popups, POPUP_GOOD, POPUP_MISS, POPUP_PERFECT } from "./hud-popups";
import { Scoreboard } from "./hud-score";
import { TrainingHud } from "./hud-training";
import { EventKind, Mode, Phase, Quality, SimEvent, Snap } from "./snapshot";

const NAMES = ["Local", "Visita"] as const; // team colors come from the theme, so no color names
const HELP_TEXT =
  "WASD mover · Mouse cámara · Rueda zoom · Click golpe · Click der. lob · Espacio salto · " +
  "Q dash · Ctrl mouse libre · Esc pausa · Mando: stick izq mover, der cámara, RT golpe, LT globo, A salto, LB/RB dash";
const LOBBY_HELP = "WASD mover · Mouse cámara · Espacio salto · Ctrl mouse libre · Esc menú";
const LAUNCH_KEY = "B"; // mirrors main.ts FEED_KEY (KeyB): Match.Feed() in Entrenamiento
const PLAYGROUND_HELP = ` · ${LAUNCH_KEY} lanzar bola`;
const CALL_MS = 1800;
const SERVE_HINT = "Click: soltar · picá detrás de la línea · soltá para sacar al cuadro cruzado";
const HUMAN_TEAM = 0;
const HUMAN_SLOT = 0; // mirrors loop.HUMAN_SLOT (single local human)
const UNSET = -1;
const MS_PER_S = 1000;
const DEFAULT_DASH_COOLDOWN_S = 0.8; // replaced by the live Config value via setDashCooldown

/** DOM overlay. Only reflects what the sim says; touches the DOM only on change. */
export class Hud {
  private readonly msg = el("msg");
  private readonly err = el("err");
  private readonly help = el("help");
  private readonly fps = el("fps");
  private readonly crosshair = el("crosshair");
  private readonly call = el("call");
  private readonly hint = el("hint");
  private readonly calls = new CallTracker();
  private readonly board = new Scoreboard();
  private readonly banner = new Callouts();
  private readonly popups = new Popups();
  private readonly training = new TrainingHud(LAUNCH_KEY);
  private readonly actions: ActionBar;
  private callUntil = 0;
  private lastPhase = UNSET;
  private lastServer = UNSET;
  private lastMode = UNSET;
  private lastFps = UNSET;
  private dashAt = -Infinity;
  private dashCooldownMs = DEFAULT_DASH_COOLDOWN_S * MS_PER_S;

  constructor(private readonly root: HTMLElement) {
    this.help.textContent = HELP_TEXT;
    this.fps.hidden = true;
    this.call.hidden = true;
    this.hint.hidden = true;
    this.hint.textContent = SERVE_HINT;
    applyThemeCss(root);
    this.crosshair.hidden = true;
    this.actions = new ActionBar();
    root.append(
      this.board.score, this.board.shots, this.msg, this.banner.root, this.call, this.hint,
      this.popups.root, this.training.root, this.training.pop, this.crosshair, this.actions.root, this.fps, this.help, this.err,
    );
  }

  setScale(scale: number): void { this.root.style.setProperty("--hud-scale", String(scale)); }
  setFpsVisible(v: boolean): void { this.fps.hidden = !v; }
  /** Live Config DashCooldownTime (s), read once per match (not per frame). */
  setDashCooldown(seconds: number): void { this.dashCooldownMs = Math.max(0, seconds) * MS_PER_S; }

  setFps(fps: number): void {
    const r = Math.round(fps);
    if (r === this.lastFps) return;
    this.lastFps = r;
    this.fps.textContent = `${r} FPS`;
  }

  /** Center crosshair, shown while the pointer is locked. */
  setCrosshair(visible: boolean): void { this.crosshair.hidden = !visible; }

  /** Entrenamiento: FeedOnDemand of the session (shows the launch hint, hides the countdown). */
  setTrainingOnDemand(v: boolean): void { this.training.setOnDemand(v); }

  /** New match: drop transient banners/cooldowns. */
  resetMatch(): void {
    this.banner.clear();
    this.training.reset();
    this.dashAt = -Infinity;
    this.call.hidden = true;
  }

  onEvent(e: SimEvent, shot: ShotKindId, nowMs: number): void {
    const call = this.calls.onEvent(e);
    if (call !== null) {
      this.call.textContent = call;
      this.call.hidden = false;
      this.callUntil = nowMs + CALL_MS;
    }
    // Entrenamiento has no rival: no "PUNTO RIVAL" banner for the sim's internal point ends.
    if (this.lastMode !== Mode.Playground || e.kind !== EventKind.Point) this.banner.onEvent(e, shot, HUMAN_TEAM, nowMs);
    this.training.onEvent(e, nowMs);
    if (e.kind === EventKind.Dash && e.player === HUMAN_SLOT) this.dashAt = nowMs;
  }

  /** Floating hit text for the human's kick at normalized screen point (x, y). */
  hitPopup(quality: number, x: number, y: number, dir: number, nowMs: number): void {
    if (quality === Quality.Perfect) this.popups.spawn(POPUP_PERFECT, "perfect", x, y, dir, nowMs);
    else if (quality === Quality.Good) this.popups.spawn(POPUP_GOOD, "good", x, y, dir, nowMs);
    else this.popups.spawn(POPUP_MISS, "miss", x, y, dir, nowMs);
  }

  update(snap: Float64Array, nowMs: number): void {
    const ready = this.dashCooldownMs <= 0 ? 1 : Math.min(1, (nowMs - this.dashAt) / this.dashCooldownMs);
    this.actions.update(snap, HUMAN_SLOT, ready);
    this.board.update(snap);
    this.banner.update(nowMs);
    this.popups.update(nowMs);
    this.training.update(snap, nowMs);
    if (!this.call.hidden && nowMs > this.callUntil) this.call.hidden = true;
    const phase = snap[Snap.Phase];
    const server = snap[Snap.Server];
    const mode = snap[Snap.Mode];
    if (phase === this.lastPhase && server === this.lastServer && mode === this.lastMode) return;
    this.lastPhase = phase;
    this.lastServer = server;
    this.lastMode = mode;
    const playground = mode === Mode.Playground;
    this.root.classList.toggle("is-lobby", mode === Mode.Lobby); // lobby.md §7: no scoreboard/streak
    this.msg.textContent = playground ? "" : message(phase, server); // RACHA owns the top in Entrenamiento
    this.hint.hidden = playground || phase !== Phase.Serve || server !== HUMAN_TEAM;
    this.help.textContent = mode === Mode.Lobby ? LOBBY_HELP : playground ? HELP_TEXT + PLAYGROUND_HELP : HELP_TEXT;
  }

  error(text: string): void { this.err.textContent = text; }
}

function message(phase: number, server: number): string {
  switch (phase) {
    case Phase.Serve:
      return `Saca ${server === 0 ? NAMES[0] : NAMES[1]}`;
    case Phase.Point:
      return "¡Punto!";
    default:
      return ""; // GameOver: the result screen takes over
  }
}

function el(id: string): HTMLDivElement {
  const d = document.createElement("div");
  d.id = id;
  return d;
}
