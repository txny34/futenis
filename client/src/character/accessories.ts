import * as THREE from "three";
import { BODY } from "./config";
import { EYE_X, EYE_Y, FACE_Z, HEAD_SCALE_XZ, HEAD_Y, MOUTH_Y } from "./head";
import type { LookMaterials } from "./materials";
import type { Rig } from "./rig";
import * as S from "./shapes";
import type { CharacterStyle } from "./style";

// Face and wrist accessories (Estilos): glasses, facial hair, wristbands.
// Glasses and facial hair are a group rebuilt only when their shape changes;
// wristbands are built once and shown or hidden. Colors are materials the
// style tints (character.ts), so recoloring never rebuilds anything.

const R = BODY.headRadius;
const GLASS_Z = FACE_Z + 0.028; // just in front of the eyeballs (they stick out of the face)
const RIM = 0.034;
const WRIST_Y = -BODY.foreArm + 0.035;

export interface Accessories {
  readonly face: THREE.Group;
  readonly wrists: readonly THREE.Mesh[];
  key: string;
}

function mesh(parent: THREE.Object3D, geo: THREE.BufferGeometry, mat: THREE.Material, x: number, y: number, z: number): THREE.Mesh {
  const m = new THREE.Mesh(geo, mat);
  m.position.set(x, y, z);
  m.castShadow = true;
  parent.add(m);
  return m;
}

/** Temples from the frame's outer corner back along the head to the ear. */
function temples(g: THREE.Object3D, mat: THREE.Material, frontX: number): void {
  const earX = R * HEAD_SCALE_XZ * 1.0;
  for (const side of [-1, 1]) {
    const x0 = side * frontX, z0 = GLASS_Z - 0.012, x1 = side * earX, z1 = -0.005;
    const len = Math.hypot(x1 - x0, z1 - z0);
    const arm = mesh(g, new THREE.BoxGeometry(0.008, 0.009, len), mat, (x0 + x1) / 2, EYE_Y + 0.008, (z0 + z1) / 2);
    arm.rotation.y = Math.atan2(x1 - x0, z1 - z0);
  }
}

function glasses(g: THREE.Object3D, kind: CharacterStyle["eyewear"], m: LookMaterials): void {
  if (kind === "none") return;
  if (kind === "sport") { // one wraparound visor lens over both eyes
    const wrapR = GLASS_Z + 0.006; // a band in front of the eyeballs, wrapping toward the temples
    mesh(g, new THREE.CylinderGeometry(wrapR, wrapR, 0.046, 24, 1, true, -0.62, 1.24), m.lens, 0, EYE_Y + 0.004, 0);
    mesh(g, new THREE.CylinderGeometry(wrapR + 0.002, wrapR + 0.002, 0.01, 24, 1, true, -0.64, 1.28), m.eyewear, 0, EYE_Y + 0.03, 0);
    temples(g, m.eyewear, Math.sin(0.62) * wrapR);
    return;
  }
  for (const side of [-1, 1]) {
    if (kind === "round") {
      mesh(g, new THREE.TorusGeometry(RIM, 0.0055, 6, 18), m.eyewear, side * EYE_X, EYE_Y, GLASS_Z);
    } else { // shades: dark lenses in a frame
      mesh(g, new THREE.BoxGeometry(0.075, 0.05, 0.008), m.lens, side * EYE_X, EYE_Y - 0.002, GLASS_Z);
      mesh(g, new THREE.BoxGeometry(0.082, 0.011, 0.012), m.eyewear, side * EYE_X, EYE_Y + 0.026, GLASS_Z);
    }
  }
  mesh(g, new THREE.BoxGeometry(0.03, 0.006, 0.008), m.eyewear, 0, EYE_Y + 0.006, GLASS_Z); // bridge
  temples(g, m.eyewear, EYE_X + (kind === "round" ? RIM : 0.04));
}

function facialHair(g: THREE.Object3D, kind: CharacterStyle["facialHair"], m: LookMaterials): void {
  if (kind === "none") return;
  const z = FACE_Z - 0.005;
  if (kind === "mustache" || kind === "beard") {
    // taperCapsule hangs from 0 to -len on Y; turned a quarter it runs 0..+len on X: start at -len/2 to center it.
    const stache = mesh(g, S.taperCapsule(0.013, 0.013, 0.056), m.hair, -0.028, MOUTH_Y + 0.024, z + 0.014);
    stache.rotation.z = Math.PI / 2;
  }
  if (kind === "stubble" || kind === "beard") { // a shell around the jaw, under the mouth line
    // phi pi/2 is the face (+Z); theta from 0.68 pi starts below the mouth.
    const jaw = mesh(g, new THREE.SphereGeometry(R * 1.015, 18, 8, Math.PI * 0.24, Math.PI * 0.52, Math.PI * 0.68, Math.PI * 0.26), kind === "beard" ? m.hair : m.stubble, 0, HEAD_Y, 0);
    jaw.scale.set(HEAD_SCALE_XZ, 1, HEAD_SCALE_XZ * (kind === "beard" ? 1.06 : 1));
  }
}

export function buildAccessories(rig: Rig, headDress: THREE.Object3D, m: LookMaterials): Accessories {
  const face = new THREE.Group();
  headDress.add(face);
  const band = new THREE.CylinderGeometry(0.044, 0.042, 0.05, 12);
  const wrists = (["L", "R"] as const).map((s) => {
    const w = mesh(rig.bones[`foreArm${s}`], band, m.wrist, 0, WRIST_Y, 0);
    w.visible = false;
    return w;
  });
  return { face, wrists, key: "" };
}

/** Applies the accessory shapes of a style (colors are set by the caller). */
export function restyleAccessories(a: Accessories, style: Readonly<CharacterStyle>, m: LookMaterials): void {
  for (const w of a.wrists) w.visible = style.wristbands;
  const key = `${style.eyewear}/${style.facialHair}`;
  if (key === a.key) return;
  a.key = key;
  for (const c of [...a.face.children]) {
    a.face.remove(c);
    if (c instanceof THREE.Mesh) (c.geometry as THREE.BufferGeometry).dispose();
  }
  glasses(a.face, style.eyewear, m);
  facialHair(a.face, style.facialHair, m);
}
