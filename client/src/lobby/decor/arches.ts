import * as THREE from "three";
import { THEME } from "../../env/theme";
import { NEON_BOOST } from "./util";

// Architecture: three giant neon arches (portals) framing the showcase court,
// dark wall panels with emissive trims along the sides and back. Arch colors
// cycle through the neon tokens (per frame: three Color.lerpColors, no allocs).
const ARCH_Z = -13.5;
const ARCH_XS: readonly number[] = [-11, 0, 11];
const ARCH_HALF_W = 4.6;
const ARCH_H = 7;
const ARCH_INNER = 0.82; // inner tube scale
const TUBE_R = 0.16;
const TUBE_SEG = 80;
const TUBE_RADIAL = 8;
const CYCLE_HZ = 0; // static palette colors (photosensitivity)
const PHASE_STEP = 1 / 3;
const WALL_X = 19.5;
const WALL_BACK_Z = 14;
const WALL_H = 9;
const PANEL_W = 3.6;
const PANEL_GAP = 0.25;
const TRIM_W = 0.12;
const TRIM_H = 6;

function archCurve(halfW: number, h: number): THREE.CatmullRomCurve3 {
  const pts: THREE.Vector3[] = [new THREE.Vector3(-halfW, 0, 0)];
  const n = 12;
  const legH = h * 0.4;
  for (let i = 0; i <= n; i++) {
    const a = Math.PI - (i / n) * Math.PI;
    pts.push(new THREE.Vector3(Math.cos(a) * halfW, legH + Math.sin(a) * (h - legH), 0));
  }
  pts.push(new THREE.Vector3(halfW, 0, 0));
  return new THREE.CatmullRomCurve3(pts);
}

interface Arch { readonly mat: THREE.MeshBasicMaterial; readonly phase: number }

export class Arches {
  readonly group = new THREE.Group();
  private readonly arches: Arch[] = [];
  private readonly palette: THREE.Color[] = THEME.lobbyDecor.neon.map((c) => new THREE.Color(c).multiplyScalar(NEON_BOOST));

  constructor() {
    const outer = new THREE.TubeGeometry(archCurve(ARCH_HALF_W, ARCH_H), TUBE_SEG, TUBE_R, TUBE_RADIAL);
    const inner = new THREE.TubeGeometry(archCurve(ARCH_HALF_W * ARCH_INNER, ARCH_H * ARCH_INNER), TUBE_SEG, TUBE_R * 0.6, TUBE_RADIAL);
    ARCH_XS.forEach((x, i) => {
      const mat = new THREE.MeshBasicMaterial({ color: this.palette[i % 3].clone() });
      const a = new THREE.Mesh(outer, mat);
      const b = new THREE.Mesh(inner, mat);
      a.position.set(x, 0, ARCH_Z);
      b.position.set(x, 0, ARCH_Z + 0.4);
      this.group.add(a, b);
      this.arches.push({ mat, phase: i * PHASE_STEP });
    });
    this.buildWalls();
  }

  update(time: number): void {
    const n = this.palette.length;
    for (const a of this.arches) {
      const u = (time * CYCLE_HZ + a.phase) * n;
      const i = Math.floor(u) % n;
      a.mat.color.lerpColors(this.palette[i], this.palette[(i + 1) % n], u - Math.floor(u));
    }
  }

  /** Side + back walls: instanced dark panels and instanced emissive trims (2 draw calls). */
  private buildWalls(): void {
    const slots: Array<readonly [number, number, number]> = []; // x, z, rotY
    for (const sx of [-1, 1]) {
      for (let z = -12; z <= WALL_BACK_Z - 2; z += PANEL_W + PANEL_GAP) slots.push([sx * WALL_X, z, (-sx * Math.PI) / 2]);
    }
    for (let x = -WALL_X + 2; x <= WALL_X - 2; x += PANEL_W + PANEL_GAP) slots.push([x, WALL_BACK_Z, Math.PI]);
    const panels = new THREE.InstancedMesh(new THREE.BoxGeometry(PANEL_W, WALL_H, 0.3),
      new THREE.MeshStandardMaterial({ color: THEME.lobbyDecor.wallPanel, roughness: 0.4, metalness: 0.6 }), slots.length);
    const trims = new THREE.InstancedMesh(new THREE.BoxGeometry(TRIM_W, TRIM_H, 0.05),
      new THREE.MeshBasicMaterial({ color: new THREE.Color(THEME.lobbyDecor.wallTrim).multiplyScalar(NEON_BOOST) }), slots.length * 2);
    const m = new THREE.Matrix4();
    const q = new THREE.Quaternion();
    const p = new THREE.Vector3();
    const s = new THREE.Vector3(1, 1, 1);
    const off = new THREE.Vector3();
    slots.forEach(([x, z, ry], i) => {
      q.setFromAxisAngle(new THREE.Vector3(0, 1, 0), ry);
      panels.setMatrixAt(i, m.compose(p.set(x, WALL_H / 2, z), q, s));
      for (let k = 0; k < 2; k++) {
        off.set((k === 0 ? -1 : 1) * (PANEL_W / 2 - 0.3), 0, 0.17).applyQuaternion(q);
        trims.setMatrixAt(i * 2 + k, m.compose(p.set(x + off.x, TRIM_H / 2 + 1, z + off.z), q, s));
      }
    });
    this.group.add(panels, trims);
  }
}
