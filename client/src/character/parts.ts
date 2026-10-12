import * as THREE from "three";
import { BODY } from "./config";
import { Face, createFace } from "./face";
import { buildHairGroup } from "./hair";
import { HEAD_SCALE_XZ, HEAD_SCALE_Y, HEAD_Y } from "./head";
import { LookMaterials } from "./materials";
import { CharacterStyle, DEFAULT_STYLE } from "./style";
import { Rig } from "./rig";
import * as S from "./shapes";

// Rigid-per-bone dressing: each body piece is a plain Mesh parented to one
// bone. No skin weights to author, trivially cheap to animate, and the
// segmented toy look hides joints behind rounded caps.

const TORSO_DEPTH = 0.62;
const PELVIS_DEPTH = 0.78;
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

function dressTorso(rig: Rig, m: LookMaterials): THREE.Mesh[] {
  const b = rig.bones;
  add(b.hips, S.profileLathe(S.PELVIS_PROFILE), m.shorts, 0, 0, 0, 1, 1, PELVIS_DEPTH);
  add(b.neck, S.taperCapsule(0.055, 0.065, 0.08), m.skin, 0, 0.07, 0);
  return [
    add(b.spine, S.profileLathe(S.LOWER_TORSO_PROFILE), m.shirt, 0, 0, 0, 1, 1, TORSO_DEPTH),
    add(b.chest, S.profileLathe(S.UPPER_TORSO_PROFILE), m.shirt, 0, 0, 0, 1, 1, TORSO_DEPTH),
  ];
}

/** The parts a style changes after the build (see restyle). */
export interface Dressing {
  readonly face: Face;
  /** Everything on the head, scaled around the head center by style.headSize. */
  readonly headDress: THREE.Group;
  /** Torso shells widened by style.build. */
  readonly torso: readonly THREE.Mesh[];
  hair: THREE.Group;
}

function dressHead(rig: Rig, m: LookMaterials): { face: Face; headDress: THREE.Group; hair: THREE.Group } {
  const headDress = new THREE.Group();
  rig.bones.head.add(headDress);
  add(headDress, S.ball(BODY.headRadius, 16, 12), m.skin, 0, HEAD_Y, 0, HEAD_SCALE_XZ, HEAD_SCALE_Y, HEAD_SCALE_XZ);
  const ear = S.ball(0.034, 8, 6);
  const earX = BODY.headRadius * HEAD_SCALE_XZ * 0.97;
  add(headDress, ear, m.skin, earX, HEAD_Y, 0);
  add(headDress, ear, m.skin, -earX, HEAD_Y, 0);
  const hair = buildHairGroup(DEFAULT_STYLE, m);
  headDress.add(hair);
  return { face: createFace(headDress, m), headDress, hair };
}

/** Applies the shape side of a style: head size, build and the hair/headwear group. */
export function restyle(d: Dressing, style: Readonly<CharacterStyle>, m: LookMaterials): void {
  const s = style.headSize;
  d.headDress.scale.setScalar(s);
  d.headDress.position.y = HEAD_Y * (1 - s); // pivot on the head center
  for (const mesh of d.torso) mesh.scale.x = style.build;
  d.headDress.remove(d.hair);
  d.hair.traverse((o) => { if (o instanceof THREE.Mesh) (o.geometry as THREE.BufferGeometry).dispose(); });
  d.hair = buildHairGroup(style, m);
  d.headDress.add(d.hair);
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

/** Builds all body meshes onto the rig; returns what a style can change later. */
export function dressRig(rig: Rig, m: LookMaterials): Dressing {
  const torso = dressTorso(rig, m);
  const head = dressHead(rig, m);
  dressArm(rig, m, "L");
  dressArm(rig, m, "R");
  dressLeg(rig, m, "L");
  dressLeg(rig, m, "R");
  return { ...head, torso };
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
