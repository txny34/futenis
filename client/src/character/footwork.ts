import { Bones } from "./rig";

// Padel footwork layers (additive, like poses.ts): the side shuffle (facing
// the net, stepping sideways instead of turning to run), the skid when a run
// brakes hard, and idle fidgets (look around, stretch) so a waiting player
// never loops one bounce. k is the blend weight 0..1. No hips height here:
// the ground snap would undo it.

const TAU = Math.PI * 2;
const SHUFFLE_SPREAD = 0.32; // legs open/close sideways
const SHUFFLE_KNEE = 0.45;
const SHUFFLE_THIGH = 0.3;
const SHUFFLE_LEAN = 0.1; // into the move
const SHUFFLE_ARMS_OUT = 0.35;

const SKID_LEAN_BACK = 0.35;
const SKID_FRONT_LEG = 0.7; // braking leg reaches ahead
const SKID_BACK_KNEE = 0.9;
const SKID_ARMS_UP = 0.6;

const LOOK_YAW = 0.7;
const STRETCH_ARM = 2.6;
const STRETCH_LEAN = 0.15;

/** Side shuffle at gait phase; dir +1 = stepping to the body's left, -1 right. */
export function applyShuffle(b: Bones, phase: number, dir: number, k: number): void {
  if (k <= 0) return;
  const open = (0.5 - 0.5 * Math.cos(phase * 2)) * SHUFFLE_SPREAD * k; // two steps per cycle
  b.upLegL.rotation.z += open;
  b.upLegR.rotation.z -= open;
  b.upLegL.rotation.x -= SHUFFLE_THIGH * k;
  b.upLegR.rotation.x -= SHUFFLE_THIGH * k;
  b.legL.rotation.x += SHUFFLE_KNEE * k;
  b.legR.rotation.x += SHUFFLE_KNEE * k;
  b.spine.rotation.z -= dir * SHUFFLE_LEAN * k;
  b.upperArmL.rotation.z += SHUFFLE_ARMS_OUT * k;
  b.upperArmR.rotation.z -= SHUFFLE_ARMS_OUT * k;
}

/** Hard stop: lean back, front leg braced ahead, back knee bent, arms up for balance. */
export function applySkid(b: Bones, k: number): void {
  if (k <= 0) return;
  b.spine.rotation.x -= SKID_LEAN_BACK * k;
  b.upLegR.rotation.x -= SKID_FRONT_LEG * k;
  b.upLegL.rotation.x += SKID_FRONT_LEG * 0.3 * k;
  b.legL.rotation.x += SKID_BACK_KNEE * k;
  b.upperArmL.rotation.z += SKID_ARMS_UP * k;
  b.upperArmR.rotation.z -= SKID_ARMS_UP * k;
}

export type Fidget = "look" | "stretch";

/** Idle fidget at progress u 0..1 (a sine bump in and out), weight k. */
export function applyFidget(b: Bones, kind: Fidget, u: number, k: number): void {
  if (k <= 0) return;
  const bump = Math.sin(Math.PI * u) * k;
  if (kind === "look") {
    b.neck.rotation.y += Math.sin(TAU * u) * LOOK_YAW * k; // left, then right
    b.chest.rotation.y += Math.sin(TAU * u) * LOOK_YAW * 0.25 * k;
    return;
  }
  b.upperArmL.rotation.z += STRETCH_ARM * bump;
  b.upperArmR.rotation.z -= STRETCH_ARM * bump;
  b.spine.rotation.x -= STRETCH_LEAN * bump;
}

const DASH_LEAN = 0.35;
const DASH_CHEST = 0.15;
const DASH_HEAD_UP = 0.35; // eyes stay on the court, not on the floor
const DASH_ARMS_BACK = 0.4; // on top of a damped run swing (animator.ts): stays behind the hip, never past the shoulder
const DASH_LEAD_THIGH = 0.8;
const DASH_LEAD_KNEE = 0.7;
const DASH_TRAIL_THIGH = 0.5;
const DASH_TRAIL_KNEE = 0.3;

/** Dash burst (sprinter's drive): chest forward, head up, arms back, lead knee driving; weight k. */
export function applyDash(b: Bones, k: number): void {
  if (k <= 0) return;
  b.spine.rotation.x += DASH_LEAN * k;
  b.chest.rotation.x += DASH_CHEST * k;
  b.neck.rotation.x -= DASH_HEAD_UP * k;
  b.upperArmL.rotation.x += DASH_ARMS_BACK * k;
  b.upperArmR.rotation.x += DASH_ARMS_BACK * k;
  b.upLegR.rotation.x -= DASH_LEAD_THIGH * k;
  b.legR.rotation.x += DASH_LEAD_KNEE * k;
  b.upLegL.rotation.x += DASH_TRAIL_THIGH * k;
  b.legL.rotation.x += DASH_TRAIL_KNEE * k;
}
