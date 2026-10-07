import * as THREE from "three";
import { THEME } from "../env/theme";

// Billboard sign above a lobby pad: canvas texture drawn ONCE at load
// (docs/design/lobby.md §7). A Sprite always faces the camera.
const CANVAS_W = 1024;
const CANVAS_H = 256;
const WORLD_W = 4.4;
const RADIUS = 48;
const BORDER = 10;
const FONT_PX = 76;
const FONT = `${FONT_PX}px "Bungee", "Impact", system-ui, sans-serif`;
const GLOW = 28;

const css = (hex: number): string => `#${hex.toString(16).padStart(6, "0")}`;

export function padSign(label: string, color: number): THREE.Sprite {
  const canvas = document.createElement("canvas");
  canvas.width = CANVAS_W;
  canvas.height = CANVAS_H;
  const ctx = canvas.getContext("2d");
  if (ctx) draw(ctx, label, css(color));
  const tex = new THREE.CanvasTexture(canvas);
  tex.colorSpace = THREE.SRGBColorSpace;
  tex.anisotropy = 4;
  const sprite = new THREE.Sprite(new THREE.SpriteMaterial({ map: tex, transparent: true, depthWrite: false }));
  sprite.scale.set(WORLD_W, (WORLD_W * CANVAS_H) / CANVAS_W, 1);
  // Web fonts may land after the first draw: redraw once they are ready.
  void document.fonts?.ready.then(() => {
    if (!ctx) return;
    draw(ctx, label, css(color));
    tex.needsUpdate = true;
  });
  return sprite;
}

function draw(ctx: CanvasRenderingContext2D, label: string, accent: string): void {
  ctx.clearRect(0, 0, CANVAS_W, CANVAS_H);
  const inset = BORDER + GLOW / 2;
  ctx.beginPath();
  ctx.roundRect(inset, inset, CANVAS_W - inset * 2, CANVAS_H - inset * 2, RADIUS);
  ctx.fillStyle = THEME.lobbySignBg;
  ctx.shadowColor = accent;
  ctx.shadowBlur = GLOW;
  ctx.fill();
  ctx.lineWidth = BORDER;
  ctx.strokeStyle = accent;
  ctx.stroke();
  ctx.shadowBlur = 0;
  ctx.fillStyle = THEME.lobbySignText;
  ctx.textAlign = "center";
  ctx.textBaseline = "middle";
  let size = FONT_PX;
  ctx.font = FONT;
  const maxW = CANVAS_W - inset * 4;
  while (ctx.measureText(label).width > maxW && size > FONT_PX / 2) {
    size -= 4;
    ctx.font = FONT.replace(`${FONT_PX}px`, `${size}px`);
  }
  ctx.fillText(label, CANVAS_W / 2, CANVAS_H / 2 + size * 0.05);
}
