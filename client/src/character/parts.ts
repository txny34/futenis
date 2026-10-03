import * as THREE from "three";
import { BODY } from "./config";
import { LookMaterials } from "./materials";
import { Rig } from "./rig";
import * as S from "./shapes";

// Rigid-per-bone dressing: each body piece is a plain Mesh parented to one
// bone. No skin weights to author, trivially cheap to animate, and the
// segmented toy look hides joints behind rounded caps.

const TORSO_DEPTH = 0.62;
const PELVIS_DEPTH = 0.78;
// Visual-only head scale (toy look). Widens/deepens; height is capped by the
// 1.80 m hitbox, so BODY heights stay untouched.
const HEAD_SCALE_XZ = 1.15;
const HEAD_SCALE_Y = 1.0;
const HEAD_Y = BODY.headCenter - BODY.headBase;
const FACE_Z = BODY.headRadius * HEAD_SCALE_XZ * 0.93;
const EYE_X = 0.055;
const EYE_Y = HEAD_Y + 0.015;
const PUPIL_FORWARD = 0.014;
const BROW_Y = EYE_Y + 0.045;
const BROW_TILT = 0.25;
const MOUTH_Y = HEAD_Y - 0.055;

function add(
  parent: THREE.Object3D, geo: THREE.BufferGeometry, mat: THREE.Material,
  x = 0, y = 0, z = 0, sx = 1, sy = 1, sz = 1,
): THREE.Mesh {
  const mesh = new THREE.Mesh(geo, mat);
  mesh.position.set(x, y, z);
  mesh.scale.set(sx, sy, sz);
  mesh.castShadow = true;
  parent.add(mesh);
  return mesh;
}

function dressTorso(rig: Rig, m: LookMaterials): void {
  const b = rig.bones;
  add(b.hips, S.profileLathe(S.PELVIS_PROFILE), m.shorts, 0, 0, 0, 1, 1, PELVIS_DEPTH);
  add(b.spine, S.profileLathe(S.LOWER_TORSO_PROFILE), m.shirt, 0, 0, 0, 1, 1, TORSO_DEPTH);
  add(b.chest, S.profileLathe(S.UPPER_TORSO_PROFILE), m.shirt, 0, 0, 0, 1, 1, TORSO_DEPTH);
  add(b.neck, S.taperCapsule(0.055, 0.065, 0.08), m.skin, 0, 0.07, 0);
}

function dressHair(head: THREE.Object3D, m: LookMaterials): void {
  const r = BODY.headRadius * 1.04;
  const cap = add(head, S.cap(r, 0.55), m.hair, 0, HEAD_Y + 0.005, -0.008, HEAD_SCALE_XZ, HEAD_SCALE_Y, HEAD_SCALE_XZ);
  cap.rotation.x = -0.55;
  const tuft = S.ball(0.05, 8, 6);
  add(head, tuft, m.hair, 0.03, HEAD_Y + 0.13, -0.02, 1.4, 0.7, 1.2).rotation.z = 0.4;
  add(head, tuft, m.hair, -0.04, HEAD_Y + 0.12, 0.03, 1.3, 0.6, 1.1).rotation.z = -0.3;
}

function dressFace(head: THREE.Object3D, m: LookMaterials): void {
  const eye = S.ball(0.03, 10, 8);
  const pupil = S.ball(0.017, 8, 6);
  const brow = new THREE.BoxGeometry(0.05, 0.012, 0.015);
  const mouth = new THREE.BoxGeometry(0.045, 0.01, 0.012);
  for (const side of [1, -1]) {
    add(head, eye, m.eyeWhite, side * EYE_X, EYE_Y, FACE_Z - 0.01, 1, 1.2, 0.5);
    add(head, pupil, m.pupil, side * EYE_X, EYE_Y, FACE_Z + PUPIL_FORWARD - 0.01, 1, 1.3, 0.5);
    const b = add(head, brow, m.hair, side * EYE_X, BROW_Y, FACE_Z - 0.012);
    b.rotation.z = -side * BROW_TILT;
  }
  add(head, mouth, m.pupil, 0, MOUTH_Y, FACE_Z - 0.022);
}

function dressHead(rig: Rig, m: LookMaterials): void {
  const head = rig.bones.head;
  add(head, S.ball(BODY.headRadius, 16, 12), m.skin, 0, HEAD_Y, 0, HEAD_SCALE_XZ, HEAD_SCALE_Y, HEAD_SCALE_XZ);
  const ear = S.ball(0.034, 8, 6);
  const earX = BODY.headRadius * HEAD_SCALE_XZ * 0.97;
  add(head, ear, m.skin, earX, HEAD_Y, 0);
  add(head, ear, m.skin, -earX, HEAD_Y, 0);
  dressHair(head, m);
  dressFace(head, m);
}

function dressArm(rig: Rig, m: LookMaterials, s: "L" | "R"): void {
  const b = rig.bones;
  add(b[`upperArm${s}` as const], S.ball(0.078, 10, 8), m.shirt, 0, -0.01, 0);
  add(b[`upperArm${s}` as const], S.taperCapsule(0.07, 0.062, 0.1), m.shirt);
  add(b[`upperArm${s}` as const], S.taperCapsule(0.053, 0.046, BODY.upperArm), m.skin);
  add(b[`foreArm${s}` as const], S.taperCapsule(0.047, 0.04, BODY.foreArm), m.skin);
  add(b[`hand${s}` as const], S.ball(0.06, 10, 8), m.skin, 0, -0.04, 0.005, 0.8, 1, 1);
}

function dressLeg(rig: Rig, m: LookMaterials, s: "L" | "R"): void {
  const b = rig.bones;
  const thigh = BODY.hip - BODY.knee;
  const shin = BODY.knee - BODY.ankle;
  add(b[`upLeg${s}` as const], S.taperCapsule(0.1, 0.09, 0.2), m.shorts);
  add(b[`upLeg${s}` as const], S.taperCapsule(0.09, 0.064, thigh), m.skin);
  add(b[`leg${s}` as const], S.taperCapsule(0.06, 0.045, shin), m.skin);
  add(b[`leg${s}` as const], S.taperCapsule(0.05, 0.05, 0.1), m.shorts, 0, -shin + 0.13, 0);
  add(b[`foot${s}` as const], S.bootShape(0.058, 0.13), m.boots, 0, -0.04, 0.055, 1.1, 0.85, 1);
  add(b[`toe${s}` as const], S.ball(0.06, 10, 6), m.boots, 0, 0.015, -0.01, 1.05, 0.75, 1.1);
}

/** Builds all body meshes onto the rig. */
export function dressRig(rig: Rig, m: LookMaterials): void {
  dressTorso(rig, m);
  dressHead(rig, m);
  dressArm(rig, m, "L");
  dressArm(rig, m, "R");
  dressLeg(rig, m, "L");
  dressLeg(rig, m, "R");
}

/** Triangle count of everything under root (for the §6 budget). */
export function countTriangles(root: THREE.Object3D): number {
  let tris = 0;
  root.traverse((o) => {
    if (!(o instanceof THREE.Mesh)) return;
    const g = o.geometry as THREE.BufferGeometry;
    tris += (g.index ? g.index.count : g.attributes.position.count) / 3;
  });
  return tris;
}
