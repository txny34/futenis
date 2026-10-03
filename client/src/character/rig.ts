import * as THREE from "three";
import { BODY } from "./config";

// Bone hierarchy for the procedural athlete. Every bone rests with identity
// rotation, so the animator can write absolute local rotations each frame.
// Character faces +Z in root space; its left side is +X. Limb bones point
// down (-Y): a NEGATIVE rotation.x swings a limb forward.

export const BONE_NAMES = [
  "hips", "spine", "chest", "neck", "head", "headTop", "brow",
  "shoulderL", "upperArmL", "foreArmL", "handL",
  "shoulderR", "upperArmR", "foreArmR", "handR",
  "upLegL", "legL", "footL", "toeL",
  "upLegR", "legR", "footR", "toeR",
] as const;

export type BoneName = (typeof BONE_NAMES)[number];
export type Bones = Readonly<Record<BoneName, THREE.Bone>>;

export interface Rig {
  readonly root: THREE.Object3D;
  readonly bones: Bones;
  readonly hipsRestY: number;
}

const SHOULDER_INSET = 0.07;
const SHOULDER_DROP = 0.02;
const HEAD_TOP_OFFSET = BODY.headCenter - BODY.headBase + BODY.headRadius;
const BROW_FORWARD = 0.12;
const BROW_UP = 0.03;
/** Standing height of the brow (header contact) socket. */
export const BROW_REST_Y = BODY.headCenter + BROW_UP;

type BoneSpec = readonly [BoneName, BoneName | null, number, number, number];

function sideSpecs(s: "L" | "R", sign: number): BoneSpec[] {
  const B = BODY;
  return [
    [`shoulder${s}`, "chest", sign * SHOULDER_INSET, B.shoulder - B.chest - SHOULDER_DROP, 0],
    [`upperArm${s}`, `shoulder${s}`, sign * (B.shoulderHalfWidth - SHOULDER_INSET), SHOULDER_DROP, 0],
    [`foreArm${s}`, `upperArm${s}`, 0, -B.upperArm, 0],
    [`hand${s}`, `foreArm${s}`, 0, -B.foreArm, 0],
    [`upLeg${s}`, "hips", sign * B.hipHalfWidth, B.hip - B.pelvis, 0],
    [`leg${s}`, `upLeg${s}`, 0, B.knee - B.hip, 0],
    [`foot${s}`, `leg${s}`, 0, B.ankle - B.knee, 0],
    [`toe${s}`, `foot${s}`, 0, B.toe - B.ankle, B.footForward],
  ];
}

function boneSpecs(): BoneSpec[] {
  const B = BODY;
  return [
    ["hips", null, 0, B.pelvis, 0],
    ["spine", "hips", 0, B.spine - B.pelvis, 0],
    ["chest", "spine", 0, B.chest - B.spine, 0],
    ["neck", "chest", 0, B.neck - B.chest, 0],
    ["head", "neck", 0, B.headBase - B.neck, 0],
    ["headTop", "head", 0, HEAD_TOP_OFFSET, 0],
    ["brow", "head", 0, B.headCenter - B.headBase + BROW_UP, BROW_FORWARD],
    ...sideSpecs("L", 1),
    ...sideSpecs("R", -1),
  ];
}

export function createRig(): Rig {
  const root = new THREE.Object3D();
  root.name = "character";
  const bones = {} as Record<BoneName, THREE.Bone>;
  for (const [name, parent, x, y, z] of boneSpecs()) {
    const bone = new THREE.Bone();
    bone.name = name;
    bone.position.set(x, y, z);
    (parent === null ? root : bones[parent]).add(bone);
    bones[name] = bone;
  }
  return { root, bones, hipsRestY: BODY.pelvis };
}

/** Resets every bone to its rest rotation/scale and the hips to rest height. */
export function resetRig(rig: Rig): void {
  for (const name of BONE_NAMES) {
    const b = rig.bones[name];
    b.rotation.set(0, 0, 0);
    b.scale.set(1, 1, 1);
  }
  rig.bones.hips.position.set(0, rig.hipsRestY, 0);
}

const ankleL = new THREE.Vector3();
const ankleR = new THREE.Vector3();
const toeL = new THREE.Vector3();
const toeR = new THREE.Vector3();

/**
 * Keeps the lowest ankle at rest height so bent-knee poses plant the feet on
 * the floor instead of floating/sinking. Call after posing, before IK.
 */
export function groundSnap(rig: Rig): void {
  rig.root.updateMatrixWorld(true);
  rig.bones.footL.getWorldPosition(ankleL);
  rig.bones.footR.getWorldPosition(ankleR);
  rig.bones.toeL.getWorldPosition(toeL);
  rig.bones.toeR.getWorldPosition(toeR);
  const rootY = rig.root.matrixWorld.elements[13];
  const ankleGap = Math.min(ankleL.y, ankleR.y) - rootY - BODY.ankle;
  const toeGap = Math.min(toeL.y, toeR.y) - rootY - BODY.toe;
  rig.bones.hips.position.y -= Math.min(ankleGap, toeGap);
}
