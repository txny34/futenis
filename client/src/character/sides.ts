import * as THREE from "three";
import { Bones } from "./rig";

// Kick-side / plant-side limb views so every strike pose is written once and
// mirrored. d = +1 for a right-side strike, -1 for left: multiply every Y/Z
// rotation (and every X offset) by d; X rotations are side-independent.

export type Side = "L" | "R";

export interface SideRig {
  readonly d: number;
  readonly kUp: THREE.Bone; readonly kLeg: THREE.Bone; readonly kFoot: THREE.Bone; readonly kToe: THREE.Bone;
  readonly pUp: THREE.Bone; readonly pLeg: THREE.Bone; readonly pFoot: THREE.Bone;
  readonly kShoulder: THREE.Bone; readonly kArm: THREE.Bone; readonly kFore: THREE.Bone;
  readonly pArm: THREE.Bone; readonly pFore: THREE.Bone;
}

export interface Sides {
  readonly R: SideRig;
  readonly L: SideRig;
}

export function createSides(b: Bones): Sides {
  return {
    R: {
      d: 1, kUp: b.upLegR, kLeg: b.legR, kFoot: b.footR, kToe: b.toeR,
      pUp: b.upLegL, pLeg: b.legL, pFoot: b.footL,
      kShoulder: b.shoulderR, kArm: b.upperArmR, kFore: b.foreArmR, pArm: b.upperArmL, pFore: b.foreArmL,
    },
    L: {
      d: -1, kUp: b.upLegL, kLeg: b.legL, kFoot: b.footL, kToe: b.toeL,
      pUp: b.upLegR, pLeg: b.legR, pFoot: b.footR,
      kShoulder: b.shoulderL, kArm: b.upperArmL, kFore: b.foreArmL, pArm: b.upperArmR, pFore: b.foreArmR,
    },
  };
}

/**
 * Side from the contact in character space. The character faces +Z, so its
 * LEFT is +X (see rig.ts): contact x > 0 -> left strike.
 */
export function sideFromLocalX(x: number): Side {
  return x > 0 ? "L" : "R";
}
