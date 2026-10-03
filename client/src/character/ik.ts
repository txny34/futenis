import * as THREE from "three";

// Analytic 2-bone IK + single-bone aim, solved in world space and written
// back as local rotations. Module scratch objects: zero allocations per call.

const EPS = 1e-4;
const REACH_SLACK = 0.999;

const pa = new THREE.Vector3();
const pb = new THREE.Vector3();
const pc = new THREE.Vector3();
const dir = new THREE.Vector3();
const bend = new THREE.Vector3();
const want = new THREE.Vector3();
const from = new THREE.Vector3();
const to = new THREE.Vector3();
const qDelta = new THREE.Quaternion();
const qWorld = new THREE.Quaternion();
const qParent = new THREE.Quaternion();
const qIdentity = new THREE.Quaternion();

/**
 * Rotates `bone` so the world vector bone->effector turns toward
 * bone->desired, blended by weight (0..1). Updates the bone's world matrices.
 */
export function aimBone(
  bone: THREE.Object3D, effector: THREE.Vector3, desired: THREE.Vector3, weight: number,
): void {
  bone.getWorldPosition(pa);
  from.subVectors(effector, pa);
  to.subVectors(desired, pa);
  if (from.lengthSq() < EPS || to.lengthSq() < EPS || weight <= 0) return;
  qDelta.setFromUnitVectors(from.normalize(), to.normalize());
  qDelta.slerp(qIdentity, 1 - Math.min(weight, 1));
  bone.getWorldQuaternion(qWorld);
  qWorld.premultiply(qDelta);
  if (bone.parent) {
    bone.parent.getWorldQuaternion(qParent);
    qWorld.premultiply(qParent.invert());
  }
  bone.quaternion.copy(qWorld);
  bone.updateMatrixWorld(true);
}

/**
 * Two-bone IK: upper -> lower -> end reaches `target`, bending toward `pole`
 * (world point). Targets out of reach are clamped (near-miss, never forced).
 */
export function solveTwoBone(
  upper: THREE.Object3D, lower: THREE.Object3D, end: THREE.Object3D,
  target: THREE.Vector3, pole: THREE.Vector3, weight: number,
): void {
  if (weight <= 0) return;
  upper.updateMatrixWorld(true);
  upper.getWorldPosition(pa);
  lower.getWorldPosition(pb);
  end.getWorldPosition(pc);
  const l1 = pa.distanceTo(pb);
  const l2 = pb.distanceTo(pc);
  dir.subVectors(target, pa);
  const dist = THREE.MathUtils.clamp(dir.length(), Math.abs(l1 - l2) + EPS, (l1 + l2) * REACH_SLACK);
  dir.normalize();
  bend.subVectors(pole, pa);
  bend.addScaledVector(dir, -bend.dot(dir));
  if (bend.lengthSq() < EPS) return;
  bend.normalize();
  const cosA = THREE.MathUtils.clamp((l1 * l1 + dist * dist - l2 * l2) / (2 * l1 * dist), -1, 1);
  const sinA = Math.sqrt(1 - cosA * cosA);
  want.copy(pa).addScaledVector(dir, l1 * cosA).addScaledVector(bend, l1 * sinA);
  aimBone(upper, pb, want, weight);
  lower.getWorldPosition(pb);
  end.getWorldPosition(pc);
  want.copy(pa).addScaledVector(dir, dist);
  aimBone(lower, pc, want, weight);
}
