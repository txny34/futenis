import { Bones } from "./rig";

// Additive base-pose layers. Each writes `+=` onto rest (identity) rotations
// scaled by k (0..1) so layers blend by weight without allocation.
// Conventions (see rig.ts): limb rotation.x < 0 swings forward; spine
// rotation.x > 0 leans forward; left arm out = +z, right arm out = -z.

const TAU = Math.PI * 2;
const ARM_REST_OUT = 0.14;
const READY_THIGH = 0.32;
const READY_KNEE = 0.62;
const READY_TOES = 0.18;
const READY_LEAN = 0.18;
const READY_ARM_FWD = 0.35;
const READY_ELBOW = 0.9;
const READY_BOUNCE_HZ = 1.6;
const READY_BOUNCE_KNEE = 0.06;
const BREATH_HZ = 0.3;
const BREATH_ROT = 0.03;
const BREATH_SCALE = 0.02;

const RUN_LEG_AMP = 0.6;
const SPRINT_LEG_AMP = 0.55;
const RUN_KNEE_BASE = 0.25;
const RUN_KNEE_SWING = 1.1;
const SPRINT_KNEE_SWING = 0.6;
const RUN_ARM_AMP = 0.9;
const SPRINT_ARM_AMP = 0.5;
const RUN_ELBOW = 0.9;
const RUN_LEAN = 0.15;
const SPRINT_LEAN = 0.4;
const HIP_TWIST = 0.2;
const CHEST_COUNTER_TWIST = 0.35;
const RUN_BOUNCE = 0.04;

const AIR_TUCK_THIGH = 0.7;
const AIR_TUCK_KNEE = 1.1;
const AIR_TRAIL_THIGH = 0.2;
const AIR_TRAIL_KNEE = 0.5;
const AIR_ARMS_UP = 1.1;
const AIR_LEAN = 0.12;

/** Arms slightly out so they never clip the torso (always on). */
export function applyRest(b: Bones): void {
  b.upperArmL.rotation.z += ARM_REST_OUT;
  b.upperArmR.rotation.z -= ARM_REST_OUT;
}

/** Padel-style ready stance: bent knees, on the balls of the feet, bouncing. */
export function applyIdle(b: Bones, time: number, k: number): void {
  const bounce = Math.sin(time * TAU * READY_BOUNCE_HZ) * READY_BOUNCE_KNEE;
  const thigh = (READY_THIGH + bounce * 0.5) * k;
  const knee = (READY_KNEE + bounce) * k;
  b.upLegL.rotation.x -= thigh;
  b.upLegR.rotation.x -= thigh;
  b.legL.rotation.x += knee;
  b.legR.rotation.x += knee;
  b.footL.rotation.x += READY_TOES * k;
  b.footR.rotation.x += READY_TOES * k;
  b.spine.rotation.x += READY_LEAN * k;
  b.upperArmL.rotation.x -= READY_ARM_FWD * k;
  b.upperArmR.rotation.x -= READY_ARM_FWD * k;
  b.foreArmL.rotation.x -= READY_ELBOW * k;
  b.foreArmR.rotation.x -= READY_ELBOW * k;
  const breath = Math.sin(time * TAU * BREATH_HZ) * k;
  b.chest.rotation.x -= breath * BREATH_ROT;
  const sc = 1 + breath * BREATH_SCALE;
  b.chest.scale.set(sc, sc, sc);
  b.neck.rotation.x += breath * BREATH_ROT - READY_LEAN * k * 0.5;
}

/** Run/sprint cycle at `phase` (radians). k = speed weight, sprint = 0..1. */
export function applyLocomotion(b: Bones, phase: number, k: number, sprint: number): void {
  const s = Math.sin(phase);
  const c = Math.cos(phase);
  const leg = (RUN_LEG_AMP + SPRINT_LEG_AMP * sprint) * k;
  const knee = RUN_KNEE_SWING + SPRINT_KNEE_SWING * sprint;
  const arm = (RUN_ARM_AMP + SPRINT_ARM_AMP * sprint) * k;
  b.upLegL.rotation.x -= s * leg;
  b.upLegR.rotation.x += s * leg;
  b.legL.rotation.x += k * (RUN_KNEE_BASE + knee * Math.max(0, c));
  b.legR.rotation.x += k * (RUN_KNEE_BASE + knee * Math.max(0, -c));
  b.upperArmL.rotation.x += s * arm;
  b.upperArmR.rotation.x -= s * arm;
  b.foreArmL.rotation.x -= RUN_ELBOW * k;
  b.foreArmR.rotation.x -= RUN_ELBOW * k;
  b.spine.rotation.x += (RUN_LEAN + SPRINT_LEAN * sprint) * k;
  b.hips.rotation.y += s * HIP_TWIST * k;
  b.chest.rotation.y -= s * CHEST_COUNTER_TWIST * k;
  b.neck.rotation.x -= (RUN_LEAN + SPRINT_LEAN * sprint) * k * 0.6;
  b.hips.position.y += RUN_BOUNCE * k * (Math.abs(c) - 0.5);
}

/** Airborne tuck; verticalVel > 0 rising (lean back), < 0 falling. */
export function applyAir(b: Bones, verticalVel: number, k: number): void {
  b.upLegL.rotation.x -= AIR_TUCK_THIGH * k;
  b.legL.rotation.x += AIR_TUCK_KNEE * k;
  b.upLegR.rotation.x -= AIR_TRAIL_THIGH * k;
  b.legR.rotation.x += AIR_TRAIL_KNEE * k;
  b.upperArmL.rotation.z += AIR_ARMS_UP * k;
  b.upperArmR.rotation.z -= AIR_ARMS_UP * k;
  b.spine.rotation.x += (verticalVel > 0 ? -AIR_LEAN : AIR_LEAN) * k;
}
