import { Bones } from "./rig";

// Full-body reactions after a set or the match (additive, like poses.ts):
// cheer = arms up pumping with little hops; slump = head down, shoulders
// rolled forward, arms hanging. k is the blend weight 0..1.

const TAU = Math.PI * 2;
const CHEER_ARM_UP = 2.5; // overhead (arm out = +z left)
const CHEER_PUMP_HZ = 2.2;
const CHEER_PUMP = 0.35;
const CHEER_ELBOW = 0.5;
const CHEER_HOP = 0.09; // m
const CHEER_LEAN_BACK = 0.12;
const CHEER_CHIN_UP = 0.25;

const SLUMP_SPINE = 0.28;
const SLUMP_HEAD = 0.45;
const SLUMP_ARM_IN = 0.12;
const SLUMP_KNEE = 0.12;
const SLUMP_SWAY_HZ = 0.5;
const SLUMP_SWAY = 0.06;

/** Arms up pumping and hopping. */
export function applyCheer(b: Bones, time: number, k: number): void {
  if (k <= 0) return;
  const pump = Math.sin(time * TAU * CHEER_PUMP_HZ);
  const up = (CHEER_ARM_UP + pump * CHEER_PUMP) * k;
  b.upperArmL.rotation.z += up;
  b.upperArmR.rotation.z -= up;
  b.foreArmL.rotation.x -= CHEER_ELBOW * k;
  b.foreArmR.rotation.x -= CHEER_ELBOW * k;
  b.spine.rotation.x -= CHEER_LEAN_BACK * k;
  b.neck.rotation.x -= CHEER_CHIN_UP * k;
  b.hips.position.y += Math.abs(pump) * CHEER_HOP * k;
}

/** Dejected: head down, shoulders forward, arms hanging, a slow sway. */
export function applySlump(b: Bones, time: number, k: number): void {
  if (k <= 0) return;
  const sway = Math.sin(time * TAU * SLUMP_SWAY_HZ) * SLUMP_SWAY * k;
  b.spine.rotation.x += SLUMP_SPINE * k;
  b.spine.rotation.z += sway;
  b.neck.rotation.x += SLUMP_HEAD * k;
  b.upperArmL.rotation.z -= SLUMP_ARM_IN * k;
  b.upperArmR.rotation.z += SLUMP_ARM_IN * k;
  b.legL.rotation.x += SLUMP_KNEE * k;
  b.legR.rotation.x += SLUMP_KNEE * k;
}
