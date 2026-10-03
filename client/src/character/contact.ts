import * as THREE from "three";
import { StrikePart } from "./config";
import { aimBone, solveTwoBone } from "./ik";
import { Rig } from "./rig";
import { SideRig } from "./sides";

// Contact IK: moves the striking body part onto the sim's contact point.
// `contact` is in WORLD space. Scratch vectors are module-level (no allocs).

// Knee pole in HIPS space: in front of the knee, so it follows the pelvis
// (upright for volleys, pointing up when the bicycle rolls the hips back).
const KNEE_POLE_X = 0.1;
const KNEE_POLE_Y = -0.45;
const KNEE_POLE_Z = 1.0;
const HEAD_POLE = new THREE.Vector3(0, 1.3, -1.0);
const CHEST_FRONT = new THREE.Vector3(0, 0.08, 0.14);
const SHOULDER_X = 0.08;
const TORSO_AIM_WEIGHT = 0.6;
/** Extra passes converge the solve when squash/stretch scale is active. */
const IK_PASSES = 2;

const pole = new THREE.Vector3();
const effector = new THREE.Vector3();

function torsoAim(rig: Rig, from: THREE.Object3D, contact: THREE.Vector3, w: number): void {
  from.localToWorld(effector);
  aimBone(rig.bones.spine, effector, contact, w * TORSO_AIM_WEIGHT);
}

function solvePass(rig: Rig, x: SideRig, part: StrikePart, contact: THREE.Vector3, w: number): void {
  if (w <= 0) return;
  const b = rig.bones;
  rig.root.updateMatrixWorld(true);
  switch (part) {
    case "foot":
    case "bicycle":
      pole.set(-x.d * KNEE_POLE_X, KNEE_POLE_Y, KNEE_POLE_Z);
      b.hips.localToWorld(pole);
      solveTwoBone(x.kUp, x.kLeg, x.kToe, contact, pole, w);
      return;
    case "knee":
      x.kLeg.getWorldPosition(effector);
      aimBone(x.kUp, effector, contact, w);
      return;
    case "head":
      pole.copy(HEAD_POLE);
      rig.root.localToWorld(pole);
      solveTwoBone(b.spine, b.neck, b.brow, contact, pole, w);
      return;
    case "chest":
      effector.copy(CHEST_FRONT);
      torsoAim(rig, b.chest, contact, w);
      return;
    case "shoulder":
      effector.set(x.d * -SHOULDER_X, 0.02, 0.02);
      torsoAim(rig, x.kArm, contact, w);
      return;
  }
}

export function applyContactIk(rig: Rig, x: SideRig, part: StrikePart, contact: THREE.Vector3, w: number): void {
  for (let i = 0; i < IK_PASSES; i++) solvePass(rig, x, part, contact, w);
}
