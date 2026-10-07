import * as THREE from "three";

// Shared helpers for the lobby decor modules (all built once at load).

/** > 1 so unlit neon crosses the bloom threshold (view/post.ts). */
export const NEON_BOOST = 1.1; // halved for the calm lobby (photosensitivity)

export const boosted = (hex: number, k = NEON_BOOST): THREE.Color => new THREE.Color(hex).multiplyScalar(k);

/** Canvas + texture pair; draw() runs once now and again when web fonts land. */
export function canvasTexture(w: number, h: number, draw: (ctx: CanvasRenderingContext2D) => void): THREE.CanvasTexture {
  const canvas = document.createElement("canvas");
  canvas.width = w;
  canvas.height = h;
  const ctx = canvas.getContext("2d");
  const tex = new THREE.CanvasTexture(canvas);
  tex.colorSpace = THREE.SRGBColorSpace;
  tex.anisotropy = 8;
  if (!ctx) return tex;
  draw(ctx);
  void document.fonts?.ready.then(() => { draw(ctx); tex.needsUpdate = true; });
  return tex;
}

export const FONT_STACK = `"Bungee", "Impact", system-ui, sans-serif`;

/** Deterministic pseudo-random in [0,1) for decor layout (no Math.random: stable look). */
export function rand(seed: number): number {
  const x = Math.sin(seed * 127.1 + 311.7) * 43758.5453;
  return x - Math.floor(x);
}
