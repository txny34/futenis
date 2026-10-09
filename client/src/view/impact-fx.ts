import * as THREE from "three";
import { THEME } from "../env/theme";

// Pooled floor effects: an expanding impact ring at each contact and a
// "splat" decal where a remate lands (fades over SPLAT_S). Geometry and
// materials are built once; per frame only transforms/opacity change.
const ORDER = 9;
const FLOOR_Y = 0.015;
const POOL = 4;
const IMPACT_S = 0.35;
const IMPACT_MAX_R = 1.3;
const IMPACT_SEGMENTS = 40;
const SPLAT_S = 2;
const SPLAT_LOBES = 11;
const SPLAT_R = 0.9;
const SPLAT_JITTER = [1, 0.62, 0.9, 0.55, 1.1, 0.7, 0.95, 0.5, 1.05, 0.66, 0.85] as const; // fixed, deterministic
const SPLAT_COLOR = THEME.fx.splat;
const SPLAT_OPACITY = 0.75;
const IMPACT_COLORS: readonly number[] = THEME.fx.impact; // perfect / good / other

interface Fx { readonly mesh: THREE.Mesh<THREE.BufferGeometry, THREE.MeshBasicMaterial>; left: number }

function splatGeometry(): THREE.ShapeGeometry {
  const shape = new THREE.Shape();
  for (let i = 0; i <= SPLAT_LOBES * 2; i++) {
    const a = (i / (SPLAT_LOBES * 2)) * Math.PI * 2;
    const r = SPLAT_R * (i % 2 === 0 ? SPLAT_JITTER[(i / 2) % SPLAT_LOBES] : 0.45);
    if (i === 0) shape.moveTo(Math.cos(a) * r, Math.sin(a) * r);
    else shape.lineTo(Math.cos(a) * r, Math.sin(a) * r);
  }
  return new THREE.ShapeGeometry(shape);
}

function pool(scene: THREE.Scene, geo: THREE.BufferGeometry, color: number): Fx[] {
  const out: Fx[] = [];
  for (let i = 0; i < POOL; i++) {
    const mat = new THREE.MeshBasicMaterial({ color, transparent: true, opacity: 0, depthWrite: false });
    const mesh = new THREE.Mesh(geo, mat);
    mesh.rotation.x = -Math.PI / 2;
    mesh.renderOrder = ORDER;
    mesh.visible = false;
    scene.add(mesh);
    out.push({ mesh, left: 0 });
  }
  return out;
}

function oldest(list: Fx[]): Fx {
  let pick = list[0];
  for (const f of list) if (f.left < pick.left) pick = f;
  return pick;
}

export class ImpactFx {
  private readonly impacts: Fx[];
  private readonly splats: Fx[];

  constructor(scene: THREE.Scene) {
    this.impacts = pool(scene, new THREE.RingGeometry(0.85, 1, IMPACT_SEGMENTS), THEME.fx.impact[2]);
    this.splats = pool(scene, splatGeometry(), SPLAT_COLOR);
  }

  /** quality: sim Quality (0 perfect, 1 good, else neutral). */
  impact(x: number, y: number, z: number, quality: number): void {
    const f = oldest(this.impacts);
    f.left = IMPACT_S;
    f.mesh.position.set(x, Math.max(FLOOR_Y, y), z);
    f.mesh.material.color.setHex(IMPACT_COLORS[quality] ?? IMPACT_COLORS[2]);
    f.mesh.visible = true;
  }

  splat(x: number, z: number): void {
    const f = oldest(this.splats);
    f.left = SPLAT_S;
    f.mesh.position.set(x, FLOOR_Y, z);
    f.mesh.rotation.z = x * 3.1 + z * 1.7; // deterministic variety
    f.mesh.visible = true;
  }

  update(dt: number): void {
    for (const f of this.impacts) {
      if (!this.tick(f, dt)) continue;
      const k = 1 - f.left / IMPACT_S;
      const r = 0.2 + IMPACT_MAX_R * k;
      f.mesh.scale.set(r, r, 1);
      f.mesh.material.opacity = 1 - k;
    }
    for (const f of this.splats) {
      if (this.tick(f, dt)) f.mesh.material.opacity = SPLAT_OPACITY * (f.left / SPLAT_S);
    }
  }

  private tick(f: Fx, dt: number): boolean {
    if (f.left <= 0) return false;
    f.left = Math.max(0, f.left - dt);
    if (f.left === 0) f.mesh.visible = false;
    return f.left > 0;
  }
}
