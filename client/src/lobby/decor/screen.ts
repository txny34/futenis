import * as THREE from "three";
import { THEME } from "../../env/theme";
import { FONT_STACK, NEON_BOOST } from "./util";

// Giant scoreboard above the center arch. Canvas texture redrawn at REDRAW_HZ
// only (cheap upload); every string is precomputed, so redraws allocate no JS
// objects. Shows the showcase court score + a scrolling ticker.
const W = 1024;
const H = 400;
const WORLD_W = 7;
// Behind the showcase court's far end, framed by the center arch from spawn
// (camera ≈ (0, 3.1, 9.4), pitch -0.34, vFOV 70: top edge of view ≈ +15° up).
const POS = { x: 0, y: 6.0, z: -27.5 } as const;
const REDRAW_HZ = 12;
const TICKER_SPEED = 140; // px/s
const FRAME_W = 0.25;
const MAX_SCORE = 99;
const TICKER = "★ PISÁ UN PAD PARA JUGAR  ·  1v1 CONTRA EL BOT  ·  2v2  ·  ENTRENAMIENTO CON LANZAPELOTAS  ·  UN TOQUE POR EQUIPO  ";
const D = THEME.lobbyDecor;
const NUMS: readonly string[] = Array.from({ length: MAX_SCORE + 1 }, (_, i) => String(i));

export class Scoreboard {
  readonly group = new THREE.Group();
  private readonly ctx: CanvasRenderingContext2D | null;
  private readonly tex: THREE.CanvasTexture;
  private scoreA = 0;
  private scoreB = 0;
  private acc = 0;
  private tickerX = 0;
  private tickerW = 0;

  constructor() {
    const canvas = document.createElement("canvas");
    canvas.width = W;
    canvas.height = H;
    this.ctx = canvas.getContext("2d");
    this.tex = new THREE.CanvasTexture(canvas);
    this.tex.colorSpace = THREE.SRGBColorSpace;
    const worldH = (WORLD_W * H) / W;
    const face = new THREE.Mesh(new THREE.PlaneGeometry(WORLD_W, worldH), new THREE.MeshBasicMaterial({ map: this.tex, toneMapped: false }));
    const frame = new THREE.Mesh(new THREE.BoxGeometry(WORLD_W + FRAME_W * 2, worldH + FRAME_W * 2, 0.3),
      new THREE.MeshBasicMaterial({ color: new THREE.Color(D.neon[0]).multiplyScalar(NEON_BOOST) }));
    frame.position.z = -0.2;
    this.group.add(frame, face);
    this.group.position.set(POS.x, POS.y, POS.z);
    this.draw();
  }

  /** Called by the showcase rally when a decorative point ends. */
  setScore(a: number, b: number): void {
    this.scoreA = Math.min(a, MAX_SCORE);
    this.scoreB = Math.min(b, MAX_SCORE);
  }

  update(dt: number): void {
    this.tickerX -= TICKER_SPEED * dt;
    if (this.tickerW > 0 && this.tickerX < -this.tickerW) this.tickerX += this.tickerW;
    this.acc += dt;
    if (this.acc < 1 / REDRAW_HZ) return;
    this.acc = 0;
    this.draw();
  }

  private draw(): void {
    const c = this.ctx;
    if (!c) return;
    c.fillStyle = D.screenBg;
    c.fillRect(0, 0, W, H);
    c.textAlign = "center";
    c.textBaseline = "middle";
    c.font = `54px ${FONT_STACK}`;
    c.fillStyle = D.screenText;
    c.fillText("ARENA · EN VIVO", W / 2, 58);
    c.font = `170px ${FONT_STACK}`;
    c.fillStyle = D.screenAccentA;
    c.fillText(NUMS[this.scoreA], W * 0.27, 200);
    c.fillStyle = D.screenAccentB;
    c.fillText(NUMS[this.scoreB], W * 0.73, 200);
    c.fillStyle = D.screenText;
    c.font = `60px ${FONT_STACK}`;
    c.fillText("VS", W / 2, 200);
    c.fillStyle = D.screenAccentA;
    c.fillRect(0, H - 80, W, 4);
    c.font = `40px ${FONT_STACK}`;
    c.textAlign = "left";
    c.fillStyle = D.screenText;
    if (this.tickerW === 0) this.tickerW = c.measureText(TICKER).width;
    c.fillText(TICKER, this.tickerX, H - 38);
    c.fillText(TICKER, this.tickerX + this.tickerW, H - 38);
    this.tex.needsUpdate = true;
  }
}
