import * as THREE from "three";
import { BODY } from "./config";
import { BROW_Y, HEAD_SCALE_XZ, HEAD_SCALE_Y, HEAD_Y } from "./head";
import * as S from "./shapes";
import { CharacterStyle, HairStyle } from "./style";

// Hair and headwear as swappable groups (Schedule I's hair/hat meshes): built
// once per style change, never per frame. A cap hides the hair under it
// (except what sticks out: ponytail); a headband sits over any hair.

const R = BODY.headRadius;
const BACK_TILT = -0.78; // hairline tilted back so the brows show

interface HairMats { readonly hair: THREE.Material; readonly hat: THREE.Material }

function put(g: THREE.Object3D, geo: THREE.BufferGeometry, mat: THREE.Material, x: number, y: number, z: number, sx = 1, sy = 1, sz = 1): THREE.Mesh {
  const m = new THREE.Mesh(geo, mat);
  m.position.set(x, y, z);
  m.scale.set(sx, sy, sz);
  m.castShadow = true;
  g.add(m);
  return m;
}

function skullCap(g: THREE.Object3D, mat: THREE.Material, radius: number, coverage: number, tilt = BACK_TILT): THREE.Mesh {
  const cap = put(g, S.cap(radius, coverage), mat, 0, HEAD_Y + 0.005, -0.008, HEAD_SCALE_XZ, HEAD_SCALE_Y, HEAD_SCALE_XZ);
  cap.rotation.x = tilt;
  return cap;
}

const HAIR: Readonly<Record<HairStyle, (g: THREE.Object3D, mat: THREE.Material) => void>> = {
  short: (g, mat) => {
    skullCap(g, mat, R * 1.04, 0.55);
    const tuft = S.ball(0.05, 8, 6);
    put(g, tuft, mat, 0.03, HEAD_Y + 0.13, -0.02, 1.4, 0.7, 1.2).rotation.z = 0.4;
    put(g, tuft, mat, -0.04, HEAD_Y + 0.12, 0.03, 1.3, 0.6, 1.1).rotation.z = -0.3;
  },
  buzz: (g, mat) => { skullCap(g, mat, R * 1.012, 0.5); },
  mohawk: (g, mat) => {
    const fin = new THREE.BoxGeometry(0.04, 0.1, 0.075);
    for (let i = 0; i < 5; i++) {
      const a = 0.7 - i * 0.42; // front to back over the crown
      const m = put(g, fin, mat, 0, HEAD_Y + Math.cos(a) * R * 1.05, Math.sin(a) * R * HEAD_SCALE_XZ);
      m.rotation.x = -a;
    }
  },
  afro: (g, mat) => { put(g, S.ball(R * 1.28, 12, 10), mat, 0, HEAD_Y + 0.06, -0.035, HEAD_SCALE_XZ, 0.95, HEAD_SCALE_XZ); },
  bald: () => undefined,
  ponytail: (g, mat) => {
    skullCap(g, mat, R * 1.03, 0.52);
    const tail = put(g, S.taperCapsule(0.035, 0.025, 0.16), mat, 0, HEAD_Y + 0.06, -R * 1.12);
    tail.rotation.x = 0.5;
  },
};

function cap(g: THREE.Object3D, mat: THREE.Material): void {
  skullCap(g, mat, R * 1.09, 0.42, -0.35);
  const visor = put(g, new THREE.CylinderGeometry(0.11, 0.11, 0.012, 14, 1, false, -Math.PI / 2, Math.PI), mat, 0, BROW_Y + 0.045, R * 0.95);
  visor.rotation.x = 0.12;
  visor.rotation.y = Math.PI / 2;
}

function headband(g: THREE.Object3D, mat: THREE.Material): void {
  const band = put(g, new THREE.TorusGeometry(R * 1.02, 0.016, 6, 20), mat, 0, BROW_Y + 0.04, 0, HEAD_SCALE_XZ, HEAD_SCALE_XZ, 1);
  band.rotation.x = Math.PI / 2 + 0.12;
}

/** Hair + headwear for `style`, ready to parent to the head bone. */
export function buildHairGroup(style: CharacterStyle, m: HairMats): THREE.Group {
  const g = new THREE.Group();
  const capped = style.headwear === "cap";
  if (!capped || style.hair === "ponytail") HAIR[style.hair](g, m.hair);
  if (capped) cap(g, m.hat);
  if (style.headwear === "headband") headband(g, m.hat);
  return g;
}
