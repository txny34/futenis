import { CallTracker } from "./hud-calls";
import { EventKind, Mode, PART_NAMES, Phase, SimEvent, Snap } from "./snapshot";

const NAMES = ["Rojo", "Azul"] as const;
const HELP_TEXT =
  "WASD mover · Mouse apuntar · Click izq. patear (mantener = carga) · Click der. globo · " +
  "Rueda efecto · Espacio salto · Shift sprint · Esc pausa";
const PLAYGROUND_HELP = " · B nueva pelota";
const QUALITY_LABELS = ["¡PERFECTO!", "Bueno", "Forzado"] as const;
const PART_LABELS: Readonly<Record<string, string>> = {
  foot: "pie", knee: "rodilla", chest: "pecho", shoulder: "hombro", head: "cabeza", bicycle: "chilena", none: "",
};
const SPIN_LABELS = ["Efecto: cortado", "Efecto: plano", "Efecto: liftado"] as const;
const FLASH_MS = 900;
const CALL_MS = 1800;
const SERVE_HINT = "Click: soltar · picá detrás de la línea · soltá para sacar al cuadro cruzado";
const HUMAN_TEAM = 0;
const UNSET = -1;

/** DOM overlay. Only reflects what the sim says; touches the DOM only on change. */
export class Hud {
  private readonly score = el("score");
  private readonly msg = el("msg");
  private readonly err = el("err");
  private readonly help = el("help");
  private readonly flash = el("flash");
  private readonly fps = el("fps");
  private readonly spin = el("spin");
  private readonly call = el("call");
  private readonly hint = el("hint");
  private readonly calls = new CallTracker();
  private callUntil = 0;
  private last0 = UNSET;
  private last1 = UNSET;
  private lastPhase = UNSET;
  private lastServer = UNSET;
  private lastMode = UNSET;
  private lastSpin = UNSET;
  private lastFps = UNSET;
  private flashUntil = 0;

  constructor(private readonly root: HTMLElement) {
    this.help.textContent = HELP_TEXT;
    this.fps.hidden = true;
    this.flash.hidden = true;
    this.call.hidden = true;
    this.hint.hidden = true;
    this.hint.textContent = SERVE_HINT;
    root.append(this.score, this.msg, this.call, this.hint, this.flash, this.spin, this.fps, this.help, this.err);
  }

  setScale(scale: number): void { this.root.style.setProperty("--hud-scale", String(scale)); }
  setFpsVisible(v: boolean): void { this.fps.hidden = !v; }

  setFps(fps: number): void {
    const r = Math.round(fps);
    if (r === this.lastFps) return;
    this.lastFps = r;
    this.fps.textContent = `${r} FPS`;
  }

  setSpin(level: number): void {
    if (level === this.lastSpin) return;
    this.lastSpin = level;
    this.spin.textContent = SPIN_LABELS[level + 1] ?? "";
  }

  onEvent(e: SimEvent, humanSlot: number, nowMs: number): void {
    const call = this.calls.onEvent(e);
    if (call !== null) {
      this.call.textContent = call;
      this.call.hidden = false;
      this.callUntil = nowMs + CALL_MS;
    }
    if ((e.kind !== EventKind.Kick && e.kind !== EventKind.BodyHit) || e.player !== humanSlot) return;
    const part = PART_LABELS[PART_NAMES[e.part] ?? "none"] ?? "";
    const q = e.kind === EventKind.Kick ? QUALITY_LABELS[e.quality] ?? "" : "Rebote";
    this.flash.textContent = part ? `${q} · ${part}` : q;
    this.flash.dataset.q = String(e.kind === EventKind.Kick ? e.quality : 2);
    this.flash.hidden = false;
    this.flashUntil = nowMs + FLASH_MS;
  }

  update(snap: Float64Array, nowMs: number): void {
    if (!this.flash.hidden && nowMs > this.flashUntil) this.flash.hidden = true;
    if (!this.call.hidden && nowMs > this.callUntil) this.call.hidden = true;
    const s0 = snap[Snap.Score0];
    const s1 = snap[Snap.Score1];
    const phase = snap[Snap.Phase];
    const server = snap[Snap.Server];
    const mode = snap[Snap.Mode];
    if (s0 === this.last0 && s1 === this.last1 && phase === this.lastPhase && server === this.lastServer && mode === this.lastMode) return;
    this.last0 = s0;
    this.last1 = s1;
    this.lastPhase = phase;
    this.lastServer = server;
    this.lastMode = mode;
    const playground = mode === Mode.Playground;
    this.score.hidden = playground;
    this.score.innerHTML = `<span class="p0">${s0}</span> – <span class="p1">${s1}</span>`;
    this.msg.textContent = playground ? "Playground" : message(phase, server, s0, s1);
    this.hint.hidden = playground || phase !== Phase.Serve || server !== HUMAN_TEAM;
    this.help.textContent = playground ? HELP_TEXT + PLAYGROUND_HELP : HELP_TEXT;
  }

  error(text: string): void { this.err.textContent = text; }
}

function message(phase: number, server: number, s0: number, s1: number): string {
  switch (phase) {
    case Phase.Serve:
      return `Saca ${server === 0 ? NAMES[0] : NAMES[1]}`;
    case Phase.Point:
      return "¡Punto!";
    case Phase.GameOver:
      return `Fin — gana ${s0 > s1 ? NAMES[0] : NAMES[1]} — Esc para salir`;
    default:
      return "";
  }
}

function el(id: string): HTMLDivElement {
  const d = document.createElement("div");
  d.id = id;
  return d;
}
