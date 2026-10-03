import { applyThemeCss } from "./env/theme";
import type { ShotKindId } from "./feel/shots";
import { ActionBar } from "./hud-actions";
import { Callouts } from "./hud-callouts";
import { CallTracker } from "./hud-calls";
import { Popups, POPUP_GOOD, POPUP_PERFECT } from "./hud-popups";
import { Scoreboard } from "./hud-score";
import { EventKind, Mode, Phase, Quality, SimEvent, Snap } from "./snapshot";

const NAMES = ["Local", "Visita"] as const; // team colors come from the theme, so no color names
const HELP_TEXT =
  "WASD mover · Mouse apuntar · Click izq. patear (mantener = carga) · Click der. globo · " +
  "Rueda efecto · Espacio salto · Shift sprint · Q dash · Esc pausa";
const PLAYGROUND_HELP = " · B nueva pelota";
const SPIN_LABELS = ["Cortado", "Plano", "Liftado"] as const;
const SPIN_KEY = "Rueda";
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
  private readonly spin = el("spin");
  private readonly call = el("call");
  private readonly hint = el("hint");
  private readonly calls = new CallTracker();
  private readonly board = new Scoreboard();
  private readonly banner = new Callouts();
  private readonly popups = new Popups();
  private readonly spinText = document.createElement("span");
  private readonly actions: ActionBar;
  private callUntil = 0;
  private lastPhase = UNSET;
  private lastServer = UNSET;
  private lastMode = UNSET;
  private lastSpin = UNSET;
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
    this.buildSpinChip();
    this.actions = new ActionBar(this.spin);
    root.append(
      this.board.score, this.board.shots, this.msg, this.banner.root, this.call, this.hint,
      this.popups.root, this.actions.root, this.fps, this.help, this.err,
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

  setSpin(level: number): void {
    if (level === this.lastSpin) return;
    this.lastSpin = level;
    this.spinText.textContent = SPIN_LABELS[level + 1] ?? "";
    this.spin.dataset.spin = String(level);
  }

  /** New match: drop transient banners/cooldowns. */
  resetMatch(): void {
    this.banner.clear();
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
    this.banner.onEvent(e, shot, HUMAN_TEAM, nowMs);
    if (e.kind === EventKind.Dash && e.player === HUMAN_SLOT) this.dashAt = nowMs;
  }

  /** Floating hit text for the human's kick at normalized screen point (x, y). */
  hitPopup(quality: number, x: number, y: number, dir: number, nowMs: number): void {
    if (quality === Quality.Perfect) this.popups.spawn(POPUP_PERFECT, "perfect", x, y, dir, nowMs);
    else if (quality === Quality.Good) this.popups.spawn(POPUP_GOOD, "good", x, y, dir, nowMs);
  }

  update(snap: Float64Array, nowMs: number): void {
    const ready = this.dashCooldownMs <= 0 ? 1 : Math.min(1, (nowMs - this.dashAt) / this.dashCooldownMs);
    this.actions.update(snap, HUMAN_SLOT, ready);
    this.board.update(snap);
    this.banner.update(nowMs);
    this.popups.update(nowMs);
    if (!this.call.hidden && nowMs > this.callUntil) this.call.hidden = true;
    const phase = snap[Snap.Phase];
    const server = snap[Snap.Server];
    const mode = snap[Snap.Mode];
    if (phase === this.lastPhase && server === this.lastServer && mode === this.lastMode) return;
    this.lastPhase = phase;
    this.lastServer = server;
    this.lastMode = mode;
    const playground = mode === Mode.Playground;
    this.msg.textContent = playground ? "Playground" : message(phase, server);
    this.hint.hidden = playground || phase !== Phase.Serve || server !== HUMAN_TEAM;
    this.help.textContent = playground ? HELP_TEXT + PLAYGROUND_HELP : HELP_TEXT;
  }

  error(text: string): void { this.err.textContent = text; }

  /** Spin indicator chip living in the action bar: icon + level + key label. */
  private buildSpinChip(): void {
    this.spin.innerHTML =
      `<svg viewBox="0 0 64 64" class="act__svg" aria-hidden="true"><circle cx="32" cy="32" r="25" class="act__track"/>` +
      `<path class="act__icon" d="M20 32 a12 12 0 1 1 12 12 M32 44 l-5 -4 M32 44 l-5 4"/></svg>`;
    this.spinText.className = "act__label";
    const key = document.createElement("kbd");
    key.className = "act__key";
    key.textContent = SPIN_KEY;
    this.spin.append(this.spinText, key);
  }
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
