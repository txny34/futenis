import * as THREE from "three";
import { THEME } from "../../env/theme";
import { FONT_STACK, canvasTexture } from "./util";

// Lobby floor: tiled panels with glowing seams and a big "FUTENIS" logo, all
// in one canvas texture (map) + a seams/logo-only emissive map. Per frame only
// emissiveIntensity breathes (scalar write).
const PX_PER_M = 40;
const TILE_M = 2;
const SEAM_PX = 3;
const BEVEL_PX = 6;
const LOGO_W_M = 9; // whole word reads from spawn
const LOGO_MAX_PX = 200;
const LOGO_Z = 1.5; // logo center (m), between spawn and pads
const ROUGHNESS = 0.55;
const METALNESS = 0.25;
const GLOW_BASE = 0.45;
const GLOW_AMP = 0.12;
const GLOW_HZ = 0.25;

const D = THEME.lobbyDecor;

function drawTiles(ctx: CanvasRenderingContext2D, w: number, h: number, emissive: boolean): void {
  const t = TILE_M * PX_PER_M;
  ctx.fillStyle = emissive ? "#000" : D.floorTile;
  ctx.fillRect(0, 0, w, h);
  for (let y = 0; y < h; y += t) {
    for (let x = 0; x < w; x += t) {
      if (!emissive) {
        const g = ctx.createLinearGradient(x, y, x + t, y + t);
        g.addColorStop(0, D.floorTileHi);
        g.addColorStop(1, D.floorTile);
        ctx.fillStyle = g;
        ctx.fillRect(x + BEVEL_PX, y + BEVEL_PX, t - BEVEL_PX * 2, t - BEVEL_PX * 2);
      }
      ctx.fillStyle = D.floorSeam;
      ctx.globalAlpha = emissive ? 0.3 : 0.45;
      ctx.shadowColor = D.floorSeam;
      ctx.shadowBlur = emissive ? 8 : 0;
      ctx.fillRect(x, y, t, SEAM_PX);
      ctx.fillRect(x, y, SEAM_PX, t);
      ctx.globalAlpha = 1;
      ctx.shadowBlur = 0;
    }
  }
}

function drawLogo(ctx: CanvasRenderingContext2D, w: number, h: number): void {
  const cy = h / 2 + LOGO_Z * PX_PER_M; // canvas down = world +Z
  ctx.save();
  ctx.translate(w / 2, cy);
  let px = LOGO_MAX_PX;
  ctx.font = `${px}px ${FONT_STACK}`;
  const target = LOGO_W_M * PX_PER_M;
  const measured = ctx.measureText("FUTENIS").width;
  if (measured > target) { px = Math.floor((px * target) / measured); ctx.font = `${px}px ${FONT_STACK}`; }
  ctx.textAlign = "center";
  ctx.textBaseline = "middle";
  ctx.lineWidth = 6;
  ctx.strokeStyle = D.floorLogo;
  ctx.shadowColor = D.floorLogo;
  ctx.shadowBlur = 30;
  ctx.strokeText("FUTENIS", 0, 0);
  ctx.beginPath();
  ctx.ellipse(0, 0, target * 0.62, px * 0.95, 0, 0, Math.PI * 2);
  ctx.lineWidth = 6;
  ctx.stroke();
  ctx.restore();
}

export interface LobbyFloor { readonly mesh: THREE.Mesh; update(time: number): void }

export function buildFloor(halfX: number, halfZ: number): LobbyFloor {
  const w = halfX * 2 * PX_PER_M;
  const h = halfZ * 2 * PX_PER_M;
  const map = canvasTexture(w, h, (ctx) => drawTiles(ctx, w, h, false));
  const glow = canvasTexture(w, h, (ctx) => { drawTiles(ctx, w, h, true); drawLogo(ctx, w, h); });
  const mat = new THREE.MeshStandardMaterial({
    map, emissiveMap: glow, emissive: 0xffffff, emissiveIntensity: GLOW_BASE, roughness: ROUGHNESS, metalness: METALNESS,
  });
  const mesh = new THREE.Mesh(new THREE.PlaneGeometry(halfX * 2, halfZ * 2), mat);
  mesh.rotation.x = -Math.PI / 2;
  mesh.position.y = 0.001;
  mesh.receiveShadow = true;
  return {
    mesh,
    update: (time) => { mat.emissiveIntensity = GLOW_BASE + Math.sin(time * Math.PI * 2 * GLOW_HZ) * GLOW_AMP; },
  };
}
