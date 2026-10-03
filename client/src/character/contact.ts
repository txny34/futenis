import * as THREE from "three";
import { StrikePart } from "./config";
import { aimBone, solveTwoBone } from "./ik";
import { Rig } from "./rig";

// Contact IK: moves the striking body part onto the sim's contact point.
// `contact` is in WORLD space. Scratch vectors are module-level (no allocs).

const KNEE_POLE = new THREE.Vector3(-0.1, 0.6, 1.2);
const BICYCLE_POLE = new THREE.Vector3(-0.1, 2.0, 1.5);
const HEAD_POLE = new THREE.Vector3(0, 1.3, -1.0);
const CHEST_FRONT = new THREE.Vector3(0, 0.08, 0.14);
const SHOULDER_POINT = new THREE.Vector3(-0.08, 0.02, 0.02);
const TORSO_AIM_WEIGHT = 0.6;

const pole = new THREE.Vector3();
const effector = new THREE.Vector3();

function torsoAim(rig: Rig, local: THREE.Vector3, from: THREE.Object3D, contact: THREE.Vector3, w: number): void {
  effector.copy(local);
  from.localToWorld(effector);
  aimBone(rig.bones.spine, effector, contact, w * TORSO_AIM_WEIGHT);
}

export function applyContactIk(rig: Rig, part: StrikePart, contact: THREE.Vector3, w: number): void {
  if (w <= 0) return;
  const b = rig.bones;
  rig.root.updateMatrixWorld(true);
  switch (part) {
    case "foot":
    case "bicycle":
      pole.copy(part === "foot" ? KNEE_POLE : BICYCLE_POLE);
      rig.root.localToWorld(pole);
      solveTwoBone(b.upLegR, b.legR, b.toeR, contact, pole, w);
      return;
    case "knee":
      b.legR.getWorldPosition(effector);
      aimBone(b.upLegR, effector, contact, w);
      return;
    case "head":
      pole.copy(HEAD_POLE);
      rig.root.localToWorld(pole);
      solveTwoBone(b.spine, b.neck, b.brow, contact, pole, w);
      return;
    case "chest":
      torsoAim(rig, CHEST_FRONT, b.chest, contact, w);
      return;
    case "shoulder":
      torsoAim(rig, SHOULDER_POINT, b.upperArmR, contact, w);
      return;
  }
}
