import * as THREE from "three";
import { THEME } from "./theme";

// Perimeter LED advertising strip: a canvas texture with "FUTENIS" repeated,
// scrolled by moving the texture offset (no re-upload, no allocations).
const BOARD_H = 0.9;
const BOARD_Y = BOARD_H / 2;
const SIDE_Z = 7; // |z| of the side boards
const SIDE_HALF_LEN = 13;
const BACK_X = 12; // board behind the rival glass
const BACK_HALF_LEN = 7;
const TEX_W = 1024;
const TEX_H = 64;
const TILE_M = 6; // meters of board per texture repeat
const SCROLL_PER_S = 0.08; // texture widths per second
const LED_BOOST = 1.6;
const FONT = `900 44px "Bungee", Impact, sans-serif`;
const WORDS = ["FUTENIS", "●", "FUTENIS", "●"] as const;

function draw(c: HTMLCanvasElement): void {
  const g = c.getContext("2d");
  if (!g) throw new Error("canvas 2d no disponible");
  g.fillStyle = THEME.ledBg;
  g.fillRect(0, 0, TEX_W, TEX_H);
  g.font = FONT;
  g.textBaseline = "middle";
  g.textAlign = "center";
  const step = TEX_W / WORDS.length;
  WORDS.forEach((w, i) => {
    g.fillStyle = i % 2 === 0 ? THEME.ledText : THEME.ledAccent;
    g.fillText(w, step * (i + 0.5), TEX_H / 2 + 2);
  });
}

/** Draws now (fallback font) and again once the display font loads; clones share the source. */
function ledTexture(): THREE.CanvasTexture {
  const c = document.createElement("canvas");
  c.width = TEX_W;
  c.height = TEX_H;
  draw(c);
  const tex = new THREE.CanvasTexture(c);
  tex.wrapS = THREE.RepeatWrapping;
  tex.colorSpace = THREE.SRGBColorSpace;
  tex.anisotropy = 4;
  document.fonts.load(FONT).then(
    () => { draw(c); tex.source.needsUpdate = true; },
    () => undefined, // offline / blocked font: the fallback drawing stays
  );
  return tex;
}

export class LedBoard {
  readonly group = new THREE.Group();
  private readonly textures: THREE.CanvasTexture[] = [];

  constructor() {
    const base = ledTexture();
    const boards: ReadonlyArray<readonly [number, number, number, number]> = [
      // [centerX, centerZ, length, rotY] — faces point toward the court
      [0, -SIDE_Z, 2 * SIDE_HALF_LEN, 0],
      [0, SIDE_Z, 2 * SIDE_HALF_LEN, Math.PI],
      [BACK_X, 0, 2 * BACK_HALF_LEN, -Math.PI / 2],
    ];
    for (const [x, z, len, rotY] of boards) {
      const tex = base.clone();
      tex.repeat.set(len / TILE_M, 1);
      tex.needsUpdate = true;
      this.textures.push(tex);
      const mat = new THREE.MeshBasicMaterial({ map: tex, color: new THREE.Color(1, 1, 1).multiplyScalar(LED_BOOST) });
      const board = new THREE.Mesh(new THREE.PlaneGeometry(len, BOARD_H), mat);
      board.position.set(x, BOARD_Y, z);
      board.rotation.y = rotY;
      this.group.add(board);
    }
  }

  update(dt: number): void {
    for (let i = 0; i < this.textures.length; i++) {
      const t = this.textures[i];
      t.offset.x = (t.offset.x + dt * SCROLL_PER_S) % 1;
    }
  }
}
