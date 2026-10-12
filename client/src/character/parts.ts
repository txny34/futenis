import * as THREE from "three";
import { BODY } from "./config";
import { Face, createFace } from "./face";
import { buildHairGroup } from "./hair";
import { HandRig, buildHand } from "./hands";
import { HEAD_SCALE_XZ, HEAD_SCALE_Y, HEAD_Y } from "./head";
import { LookMaterials } from "./materials";
import { CharacterStyle, DEFAULT_STYLE } from "./style";
import { Rig } from "./rig";
import * as S from "./shapes";

// Rigid-per-bone dressing: each body piece is a plain Mesh parented to one
// bone. No skin weights to author, trivially cheap to animate, and the
// segmented toy look hides joints behind rounded caps.

const TORSO_DEPTH = 0.62;
const PELVIS_DEPTH = 0.7;
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
  add(b.neck, S.taperCapsule(0.06, 0.07, 0.07), m.skin, 0, 0.05, 0);
  const collar = add(b.neck, new THREE.TorusGeometry(0.075, 0.017, 6, 16), m.trim, 0, -0.005, 0.005, 1, 1, 0.85);
  collar.rotation.x = Math.PI / 2 + 0.25; // the trim sits lower at the front, like a crew neck
  return [
    add(b.spine, S.profileLathe(S.LOWER_TORSO_PROFILE), m.shirt, 0, 0, 0, 1, 1, TORSO_DEPTH),
    add(b.chest, S.profileLathe(S.UPPER_TORSO_PROFILE), m.shirt, 0, 0, 0, 1, 1, TORSO_DEPTH),
  ];
}

export { TORSO_DEPTH };

/** The parts a style changes after the build (see restyle). */
export interface Dressing {
  readonly face: Face;
  /** Everything on the head, scaled around the head center by style.headSize. */
  readonly headDress: THREE.Group;
  /** Torso shells widened by style.build. */
  readonly torso: readonly THREE.Mesh[];
  readonly hands: readonly HandRig[];
  hair: THREE.Group;
  /** Shape the current hair group was built for (rebuilt only when it changes). */
  hairKey: string;
}

const hairKeyOf = (s: Readonly<CharacterStyle>): string => `${s.hair}/${s.headwear}`;

function dressHead(rig: Rig, m: LookMaterials): Omit<Dressing, "torso" | "hands"> {
  const headDress = new THREE.Group();
  rig.bones.head.add(headDress);
  add(headDress, S.ball(BODY.headRadius, 16, 12), m.skin, 0, HEAD_Y, 0, HEAD_SCALE_XZ, HEAD_SCALE_Y, HEAD_SCALE_XZ);
  const ear = S.ball(0.034, 8, 6);
  const earX = BODY.headRadius * HEAD_SCALE_XZ * 0.97;
  add(headDress, ear, m.skin, earX, HEAD_Y, 0);
  add(headDress, ear, m.skin, -earX, HEAD_Y, 0);
  const hair = buildHairGroup(DEFAULT_STYLE, m);
  headDress.add(hair);
  return { face: createFace(headDress, m), headDress, hair, hairKey: hairKeyOf(DEFAULT_STYLE) };
}

/** Applies the shape side of a style: head size, build and the hair/headwear group. */
export function restyle(d: Dressing, style: Readonly<CharacterStyle>, m: LookMaterials): void {
  const s = style.headSize;
  d.headDress.scale.setScalar(s);
  d.headDress.position.y = HEAD_Y * (1 - s); // pivot on the head center
  for (const mesh of d.torso) mesh.scale.x = style.build;
  const key = hairKeyOf(style);
  if (key === d.hairKey) return; // size/build/colors only: no geometry rebuild
  d.hairKey = key;
  d.headDress.remove(d.hair);
  d.hair.traverse((o) => { if (o instanceof THREE.Mesh) (o.geometry as THREE.BufferGeometry).dispose(); });
  d.hair = buildHairGroup(style, m);
  d.headDress.add(d.hair);
}

function dressArm(rig: Rig, m: LookMaterials, s: "L" | "R"): HandRig {
  const b = rig.bones;
  const inward = s === "L" ? -1 : 1; // the body's center is -X for the left arm
  add(b[`upperArm${s}` as const], S.taperCapsule(0.066, 0.058, 0.1), m.shirt); // short sleeve over the joint
  add(b[`upperArm${s}` as const], S.taperCapsule(0.05, 0.043, BODY.upperArm), m.skin);
  add(b[`foreArm${s}` as const], S.taperCapsule(0.045, 0.034, BODY.foreArm), m.skin);
  return buildHand(b[`hand${s}` as const], m.skin, inward);
}

function dressLeg(rig: Rig, m: LookMaterials, s: "L" | "R"): void {
  const b = rig.bones;
  const thigh = BODY.hip - BODY.knee;
  const shin = BODY.knee - BODY.ankle;
  add(b[`upLeg${s}` as const], S.taperCapsule(0.102, 0.098, 0.16), m.shorts); // football shorts leg, above the knee
  add(b[`upLeg${s}` as const], S.taperCapsule(0.094, 0.068, thigh), m.skin);
  add(b[`leg${s}` as const], S.taperCapsule(0.07, 0.046, shin), m.skin); // calf taper
  add(b[`leg${s}` as const], S.taperCapsule(0.058, 0.052, 0.17), m.trim, 0, -shin + 0.2, 0); // tall sock over the calf
  dressShoe(rig, m, s);
}

/** Sneaker: colored upper, white sole slab, a lace stripe on top. */
function dressShoe(rig: Rig, m: LookMaterials, s: "L" | "R"): void {
  const b = rig.bones;
  add(b[`foot${s}` as const], S.bootShape(0.056, 0.13), m.boots, 0, -0.035, 0.055, 1.08, 0.8, 1);
  add(b[`toe${s}` as const], S.ball(0.058, 10, 6), m.boots, 0, 0.012, -0.01, 1.04, 0.72, 1.12);
  add(b[`foot${s}` as const], S.bootShape(0.058, 0.17), m.sole, 0, -0.078, 0.07, 1.12, 0.3, 1.06);
  const laces = add(b[`foot${s}` as const], new THREE.BoxGeometry(0.04, 0.008, 0.07), m.sole, 0, 0.012, 0.075);
  laces.rotation.x = -0.35;
}

/** Builds all body meshes onto the rig; returns what a style can change later. */
export function dressRig(rig: Rig, m: LookMaterials): Dressing {
  const torso = dressTorso(rig, m);
  const head = dressHead(rig, m);
  const hands = [dressArm(rig, m, "L"), dressArm(rig, m, "R")];
  dressLeg(rig, m, "L");
  dressLeg(rig, m, "R");
  return { ...head, torso, hands };
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
