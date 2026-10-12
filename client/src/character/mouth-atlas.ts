import * as THREE from "three";
import { THEME } from "../env/theme";
import { MOUTH_CELLS, Mouth } from "./expressions";

// One shared canvas atlas with every mouth (a strip of MOUTH_CELLS cells).
// Each face clones the texture (same image, own offset) and picks its cell by
// moving offset.x: swapping an expression allocates nothing and uploads nothing.

const CELL_W = 128;
const CELL_H = 64;
const LINE = 7;
const DRAW_SCALE = 1.35;

const css = (hex: number): string => `#${hex.toString(16).padStart(6, "0")}`;
const INK = css(THEME.fx.mouth);
const TEETH = css(THEME.fx.teeth);
const TONGUE = css(THEME.fx.tongue);

type Draw = (g: CanvasRenderingContext2D) => void;

function stroke(g: CanvasRenderingContext2D, path: () => void): void {
  g.beginPath();
  path();
  g.lineWidth = LINE;
  g.lineCap = "round";
  g.strokeStyle = INK;
  g.stroke();
}

function filled(g: CanvasRenderingContext2D, path: () => void, color: string): void {
  g.beginPath();
  path();
  g.fillStyle = color;
  g.fill();
}

// Cells are drawn in local coordinates: 0..CELL_W x 0..CELL_H, centered at (64, 32).
const CELLS: Readonly<Record<number, Draw>> = {
  [Mouth.Neutral]: (g) => stroke(g, () => { g.moveTo(44, 34); g.lineTo(84, 34); }),
  [Mouth.Smile]: (g) => stroke(g, () => { g.moveTo(38, 26); g.quadraticCurveTo(64, 50, 90, 26); }),
  [Mouth.Grin]: (g) => {
    filled(g, () => { g.moveTo(32, 20); g.quadraticCurveTo(64, 64, 96, 20); g.closePath(); }, INK);
    filled(g, () => { g.moveTo(38, 22); g.lineTo(90, 22); g.quadraticCurveTo(64, 34, 38, 22); }, TEETH);
  },
  [Mouth.Frown]: (g) => stroke(g, () => { g.moveTo(40, 44); g.quadraticCurveTo(64, 20, 88, 44); }),
  [Mouth.Oh]: (g) => filled(g, () => g.ellipse(64, 33, 13, 17, 0, 0, Math.PI * 2), INK),
  [Mouth.Gritted]: (g) => {
    filled(g, () => g.roundRect(38, 22, 52, 22, 8), INK);
    filled(g, () => g.roundRect(42, 25, 44, 16, 5), TEETH);
    stroke(g, () => { g.moveTo(42, 33); g.lineTo(86, 33); });
  },
  [Mouth.Smirk]: (g) => stroke(g, () => { g.moveTo(42, 36); g.quadraticCurveTo(70, 40, 88, 24); }),
  [Mouth.Shout]: (g) => {
    filled(g, () => g.ellipse(64, 32, 26, 22, 0, 0, Math.PI * 2), INK);
    filled(g, () => g.ellipse(64, 44, 15, 9, 0, 0, Math.PI * 2), TONGUE);
    filled(g, () => g.roundRect(46, 12, 36, 7, 3), TEETH);
  },
};

let shared: THREE.CanvasTexture | null = null;

function atlas(): THREE.CanvasTexture {
  if (shared) return shared;
  const canvas = document.createElement("canvas");
  canvas.width = CELL_W * MOUTH_CELLS;
  canvas.height = CELL_H;
  const g = canvas.getContext("2d");
  if (g) {
    for (let i = 0; i < MOUTH_CELLS; i++) {
      g.save();
      // Drawn ~1.35x around the cell center: mouths must read at gameplay distance.
      g.translate(i * CELL_W + CELL_W / 2, CELL_H / 2);
      g.scale(DRAW_SCALE, DRAW_SCALE);
      g.translate(-CELL_W / 2, -CELL_H / 2);
      CELLS[i](g);
      g.restore();
    }
  }
  const tex = new THREE.CanvasTexture(canvas);
  tex.colorSpace = THREE.SRGBColorSpace;
  tex.repeat.set(1 / MOUTH_CELLS, 1);
  shared = tex;
  return tex;
}

/** A per-face view of the shared atlas: same image, its own cell offset. */
export function mouthTexture(): THREE.Texture {
  const tex = atlas().clone();
  tex.repeat.set(1 / MOUTH_CELLS, 1);
  return tex;
}

/** Shows `cell` on a texture from mouthTexture(). */
export function showMouth(tex: THREE.Texture, cell: number): void {
  tex.offset.x = cell / MOUTH_CELLS;
}
