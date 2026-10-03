import * as THREE from "three";
import { THEME } from "./theme";

// Roof truss + four corner light towers. Every beam is one instance of a unit
// box (single draw call); lamp heads are emissive so the bloom picks them up.
const ROOF_Y = 15;
const ROOF_HALF_X = 18;
const ROOF_HALF_Z = 12.5;
const TRUSS_DEPTH = 1.4;
const BAY = 4; // meters between cross beams
const BEAM = 0.18;
const TOWER_X = 17;
const TOWER_Z = 12.5;
const TOWER_H = 17;
const TOWER_W = 0.35;
const LAMP_W = 2.6;
const LAMP_H = 1.2;
const LAMP_D = 0.25;
const LAMP_BOOST = 2.4; // > 1 so the lamp faces exceed the bloom threshold

type Seg = readonly [THREE.Vector3, THREE.Vector3];

const v = (x: number, y: number, z: number): THREE.Vector3 => new THREE.Vector3(x, y, z);

function roofSegments(): Seg[] {
  const segs: Seg[] = [];
  const top = ROOF_Y + TRUSS_DEPTH;
  for (const z of [-ROOF_HALF_Z, ROOF_HALF_Z]) {
    segs.push([v(-ROOF_HALF_X, ROOF_Y, z), v(ROOF_HALF_X, ROOF_Y, z)]);
    segs.push([v(-ROOF_HALF_X, top, z), v(ROOF_HALF_X, top, z)]);
  }
  for (let x = -ROOF_HALF_X; x <= ROOF_HALF_X + 1e-6; x += BAY) {
    segs.push([v(x, ROOF_Y, -ROOF_HALF_Z), v(x, ROOF_Y, ROOF_HALF_Z)]);
    segs.push([v(x, top, -ROOF_HALF_Z), v(x, top, ROOF_HALF_Z)]);
    for (const z of [-ROOF_HALF_Z, ROOF_HALF_Z]) {
      segs.push([v(x, ROOF_Y, z), v(x, top, z)]);
      if (x + BAY <= ROOF_HALF_X + 1e-6) segs.push([v(x, ROOF_Y, z), v(x + BAY, top, z)]);
    }
  }
  for (const sx of [-1, 1]) {
    for (const sz of [-1, 1]) segs.push([v(sx * TOWER_X, 0, sz * TOWER_Z), v(sx * TOWER_X, TOWER_H, sz * TOWER_Z)]);
  }
  return segs;
}

function beams(segs: readonly Seg[]): THREE.InstancedMesh {
  const mat = new THREE.MeshLambertMaterial({ color: THEME.truss });
  const inst = new THREE.InstancedMesh(new THREE.BoxGeometry(1, 1, 1), mat, segs.length);
  const up = v(0, 1, 0);
  const dir = new THREE.Vector3();
  const q = new THREE.Quaternion();
  const s = new THREE.Vector3();
  const mid = new THREE.Vector3();
  const m = new THREE.Matrix4();
  segs.forEach(([a, b], i) => {
    dir.subVectors(b, a);
    const len = dir.length();
    q.setFromUnitVectors(up, dir.normalize());
    const w = b.y - a.y > TOWER_H - 1 ? TOWER_W : BEAM;
    s.set(w, len, w);
    mid.addVectors(a, b).multiplyScalar(0.5);
    inst.setMatrixAt(i, m.compose(mid, q, s));
  });
  return inst;
}

/** Lamp heads at the top of each tower, facing the court center. */
function lamps(): THREE.Group {
  const g = new THREE.Group();
  const mat = new THREE.MeshBasicMaterial({ color: new THREE.Color(THEME.lampHead).multiplyScalar(LAMP_BOOST), fog: false });
  const geo = new THREE.BoxGeometry(LAMP_W, LAMP_H, LAMP_D);
  for (const sx of [-1, 1]) {
    for (const sz of [-1, 1]) {
      const lamp = new THREE.Mesh(geo, mat);
      lamp.position.set(sx * TOWER_X, TOWER_H, sz * TOWER_Z);
      lamp.lookAt(0, 0, 0);
      g.add(lamp);
    }
  }
  return g;
}

/** World positions of the lamp heads, for the lighting rig. */
export const LAMP_POSITIONS: ReadonlyArray<readonly [number, number, number]> = [
  [-TOWER_X, TOWER_H, -TOWER_Z], [-TOWER_X, TOWER_H, TOWER_Z], [TOWER_X, TOWER_H, -TOWER_Z], [TOWER_X, TOWER_H, TOWER_Z],
];

export function buildStructure(): THREE.Group {
  const g = new THREE.Group();
  g.add(beams(roofSegments()), lamps());
  return g;
}
