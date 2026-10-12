import * as THREE from "three";
import { UPPER_TORSO_PROFILE } from "./shapes";

// The back of the shirt: name over number, drawn on a canvas and mapped on a
// thin open shell that follows the upper torso (shapes.ts UPPER_TORSO_PROFILE)
// just outside it. Redrawn only when the name, number or ink change.

const W = 256;
const H = 256;
const BOTTOM_Y = -0.05; // chest-local band the print covers (about 26 cm, like a real shirt number)
const TOP_Y = 0.21;
const ROWS = 9; // evenly spaced in height so the print is not stretched
const CLEARANCE = 0.005;
const ARC = 1.3; // radians of the back covered (centered on -Z)
const NAME_PX = 36;
const NUMBER_PX = 170;
const MAX_NAME = 12;

export interface Jersey {
  readonly mesh: THREE.Mesh;
  /** number 0..99; an empty name prints the number only. */
  set(number: number, name: string, ink: number): void;
  dispose(): void;
}

export function createJersey(chest: THREE.Object3D, depth: number): Jersey {
  const canvas = document.createElement("canvas");
  canvas.width = W;
  canvas.height = H;
  const ctx = canvas.getContext("2d");
  const texture = new THREE.CanvasTexture(canvas);
  texture.colorSpace = THREE.SRGBColorSpace;
  const material = new THREE.MeshBasicMaterial({ map: texture, transparent: true, depthWrite: false });
  const geo = new THREE.LatheGeometry(band(), 16, Math.PI - ARC / 2, ARC);
  const mesh = new THREE.Mesh(geo, material);
  mesh.scale.z = depth;
  mesh.renderOrder = 1; // over the shirt it sits on
  chest.add(mesh);
  let key = "";
  return {
    mesh,
    set(number, name, ink) {
      const n = Math.max(0, Math.min(99, Math.round(number)));
      const label = Array.from(name.trim()).slice(0, MAX_NAME).join("").toUpperCase(); // code points: no split emoji
      const next = `${n}|${label}|${ink}`;
      if (next === key || !ctx) return;
      key = next;
      draw(ctx, n, label, `#${ink.toString(16).padStart(6, "0")}`);
      texture.needsUpdate = true;
    },
    dispose() {
      geo.dispose();
      material.dispose();
      texture.dispose();
    },
  };
}

function draw(ctx: CanvasRenderingContext2D, n: number, name: string, ink: string): void {
  ctx.clearRect(0, 0, W, H);
  ctx.fillStyle = ink;
  ctx.textAlign = "center";
  ctx.textBaseline = "alphabetic";
  if (name) {
    ctx.font = `800 ${NAME_PX}px system-ui, sans-serif`;
    ctx.fillText(name, W / 2, NAME_PX + 6, W - 40);
  }
  ctx.font = `900 ${NUMBER_PX}px system-ui, sans-serif`;
  ctx.fillText(String(n), W / 2, H - 22, W - 40);
}

/** The torso profile's radius at y (linear between its points). */
function radiusAt(y: number): number {
  const p = UPPER_TORSO_PROFILE;
  for (let i = 1; i < p.length; i++) {
    const [r0, y0] = p[i - 1];
    const [r1, y1] = p[i];
    if (y >= y0 && y <= y1 && y1 > y0) return r0 + ((y - y0) / (y1 - y0)) * (r1 - r0);
  }
  return p[0][0];
}

function band(): THREE.Vector2[] {
  return Array.from({ length: ROWS }, (_, j) => {
    const y = BOTTOM_Y + ((TOP_Y - BOTTOM_Y) * j) / (ROWS - 1);
    return new THREE.Vector2(radiusAt(y) + CLEARANCE, y);
  });
}
