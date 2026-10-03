import { Bones } from "./rig";

// Additive base-pose layers. Each writes `+=` onto rest (identity) rotations
// scaled by k (0..1) so layers blend by weight without allocation.
// Conventions (see rig.ts): limb rotation.x < 0 swings forward; spine
// rotation.x > 0 leans forward; left arm out = +z, right arm out = -z.

const TAU = Math.PI * 2;
const ARM_REST_OUT = 0.14;
const IDLE_KNEE = 0.08;
const BREATH_HZ = 0.3;
const BREATH_ROT = 0.03;
const BREATH_SCALE = 0.02;

const RUN_LEG_AMP = 0.55;
const SPRINT_LEG_AMP = 0.35;
const RUN_KNEE_BASE = 0.2;
const RUN_KNEE_SWING = 1.0;
const SPRINT_KNEE_SWING = 0.5;
const RUN_ARM_AMP = 0.6;
const SPRINT_ARM_AMP = 0.35;
const RUN_ELBOW = 0.5;
const RUN_LEAN = 0.12;
const SPRINT_LEAN = 0.22;
const RUN_TWIST = 0.15;
const RUN_BOUNCE = 0.04;

const AIR_TUCK_THIGH = 0.7;
const AIR_TUCK_KNEE = 1.1;
const AIR_TRAIL_THIGH = 0.2;
const AIR_TRAIL_KNEE = 0.5;
const AIR_ARMS_UP = 1.1;
const AIR_LEAN = 0.12;

/** Always-on stance: arms slightly out, soft knees, breathing (fades with k). */
export function applyIdle(b: Bones, time: number, k: number): void {
  b.upperArmL.rotation.z += ARM_REST_OUT;
  b.upperArmR.rotation.z -= ARM_REST_OUT;
  b.upLegL.rotation.x -= IDLE_KNEE * k;
  b.upLegR.rotation.x -= IDLE_KNEE * k;
  b.legL.rotation.x += 2 * IDLE_KNEE * k;
  b.legR.rotation.x += 2 * IDLE_KNEE * k;
  const breath = Math.sin(time * TAU * BREATH_HZ) * k;
  b.chest.rotation.x -= breath * BREATH_ROT;
  b.chest.scale.set(1 + breath * BREATH_SCALE, 1 + breath * BREATH_SCALE, 1 + breath * BREATH_SCALE);
  b.neck.rotation.x += breath * BREATH_ROT;
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
  b.foreArmL.rotation.x -= RUN_ELBOW * (1 + sprint) * k;
  b.foreArmR.rotation.x -= RUN_ELBOW * (1 + sprint) * k;
  b.spine.rotation.x += (RUN_LEAN + SPRINT_LEAN * sprint) * k;
  b.spine.rotation.y += s * RUN_TWIST * k;
  b.chest.rotation.y -= s * RUN_TWIST * k;
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
