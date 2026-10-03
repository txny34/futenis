import { Phase, Snap } from "./snapshot";

const NAMES = ["Rojo", "Azul"] as const;
const HELP_HTML =
  "Rojo (abajo): W/S adelante/atrás · A/D izq/der · F patear (mantener = potencia) · G globo<br>" +
  "Azul (arriba): Flechas en pantalla (↑ hacia arriba) · K patear · L globo<br>R reiniciar";
const UNSET = -1;

/** DOM overlay. Only reflects what the snapshot says; touches the DOM only on change. */
export class Hud {
  private readonly score: HTMLDivElement;
  private readonly msg: HTMLDivElement;
  private readonly err: HTMLDivElement;
  private last0 = UNSET;
  private last1 = UNSET;
  private lastPhase = UNSET;
  private lastServer = UNSET;

  constructor(root: HTMLElement) {
    this.score = el("score");
    this.msg = el("msg");
    this.err = el("err");
    const help = el("help");
    help.innerHTML = HELP_HTML;
    root.append(this.score, this.msg, help, this.err);
  }

  update(snap: Float64Array): void {
    const s0 = snap[Snap.Score0];
    const s1 = snap[Snap.Score1];
    const phase = snap[Snap.Phase];
    const server = snap[Snap.Server];
    if (s0 === this.last0 && s1 === this.last1 && phase === this.lastPhase && server === this.lastServer) return;
    this.last0 = s0;
    this.last1 = s1;
    this.lastPhase = phase;
    this.lastServer = server;
    this.score.innerHTML = `<span class="p0">${s0}</span> – <span class="p1">${s1}</span>`;
    this.msg.textContent = message(phase, server, s0, s1);
  }

  error(text: string): void {
    this.err.textContent = text;
  }
}

function message(phase: number, server: number, s0: number, s1: number): string {
  switch (phase) {
    case Phase.Serve:
      return `Saca ${server === 0 ? NAMES[0] : NAMES[1]}`;
    case Phase.Point:
      return "¡Punto!";
    case Phase.GameOver:
      return `Fin — gana ${s0 > s1 ? NAMES[0] : NAMES[1]} — R para reiniciar`;
    default:
      return "";
  }
}

function el(id: string): HTMLDivElement {
  const d = document.createElement("div");
  d.id = id;
  return d;
}
