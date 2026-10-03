import * as THREE from "three";
import { THEME } from "./theme";

// Big abstract neon shapes in the background: tubes along curves, unlit and
// boosted above 1.0 so the bloom pass makes them glow. Built once; the only
// per-frame work is one shared uniform-free intensity pulse (a scalar copy).
const TUBE_RADIUS = 0.12;
const TUBE_SEGMENTS = 96;
const TUBE_RADIAL = 6;
const NEON_BOOST = 2.2;
const PULSE_HZ = 0.35;
const PULSE_AMOUNT = 0.18;
const BACK_X = 24;

const v = (x: number, y: number, z: number): THREE.Vector3 => new THREE.Vector3(x, y, z);

interface NeonShape { readonly points: readonly THREE.Vector3[]; readonly closed: boolean; readonly color: "a" | "b" }

/** Ribbon behind the rival stand, two crossing arcs above the side stands, a ring "sun". */
function shapes(): NeonShape[] {
  const wave: THREE.Vector3[] = [];
  for (let i = 0; i <= 8; i++) {
    const z = -18 + i * 4.5;
    wave.push(v(BACK_X, 10 + Math.sin(i * 0.9) * 2.2, z));
  }
  const ring: THREE.Vector3[] = [];
  for (let i = 0; i < 12; i++) {
    const a = (i / 12) * Math.PI * 2;
    ring.push(v(BACK_X + 1, 13 + Math.sin(a) * 3.2, Math.cos(a) * 3.2));
  }
  const arc = (z: number): THREE.Vector3[] => [v(-20, 6, z), v(-8, 12, z * 1.05), v(8, 12, z * 1.05), v(20, 6, z)];
  return [
    { points: wave, closed: false, color: "a" },
    { points: ring, closed: true, color: "b" },
    { points: arc(-19), closed: false, color: "b" },
    { points: arc(19), closed: false, color: "a" },
  ];
}

export class Neon {
  readonly group = new THREE.Group();
  private readonly matA: THREE.MeshBasicMaterial;
  private readonly matB: THREE.MeshBasicMaterial;
  private readonly baseA = new THREE.Color();
  private readonly baseB = new THREE.Color();
  private time = 0;

  constructor() {
    this.baseA.setHex(THEME.neonA).multiplyScalar(NEON_BOOST);
    this.baseB.setHex(THEME.neonB).multiplyScalar(NEON_BOOST);
    this.matA = new THREE.MeshBasicMaterial({ color: this.baseA.clone(), fog: false });
    this.matB = new THREE.MeshBasicMaterial({ color: this.baseB.clone(), fog: false });
    for (const s of shapes()) {
      const curve = new THREE.CatmullRomCurve3(s.points.slice(), s.closed);
      const geo = new THREE.TubeGeometry(curve, TUBE_SEGMENTS, TUBE_RADIUS, TUBE_RADIAL, s.closed);
      this.group.add(new THREE.Mesh(geo, s.color === "a" ? this.matA : this.matB));
    }
  }

  /** Slow counter-phase breathing between the two neon colors. Allocation-free. */
  update(dt: number): void {
    this.time += dt;
    const k = Math.sin(this.time * Math.PI * 2 * PULSE_HZ) * PULSE_AMOUNT;
    this.matA.color.copy(this.baseA).multiplyScalar(1 + k);
    this.matB.color.copy(this.baseB).multiplyScalar(1 - k);
  }
}
