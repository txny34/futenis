import { StrikePart } from "./config";
import { Bones } from "./rig";

// Per-part strike poses (right side strikes). `s` is the swing: -1 full
// wind-up, 0 impact, +1 full follow-through. `w` blends the whole pose.
// The same function renders the charge wind-up (s<0) and the strike.

function sw(s: number, wind: number, follow: number): number {
  return s < 0 ? -s * wind : s * follow;
}

function armsOut(b: Bones, w: number, amount: number): void {
  b.upperArmL.rotation.z += w * amount;
  b.upperArmR.rotation.z -= w * amount;
}

function foot(b: Bones, s: number, w: number): void {
  b.upLegR.rotation.x += w * sw(s, 0.8, -1.5);
  b.legR.rotation.x += w * sw(s, 1.5, 0.1);
  b.upLegL.rotation.x += w * 0.1;
  b.legL.rotation.x += w * 0.25;
  b.spine.rotation.x -= w * 0.15;
  b.spine.rotation.y += w * sw(s, 0.35, -0.3);
  armsOut(b, w, 0.7);
}

function knee(b: Bones, s: number, w: number): void {
  b.upLegR.rotation.x += w * sw(s, 0.3, -1.7);
  b.legR.rotation.x += w * sw(s, 0.8, 2.0);
  b.legL.rotation.x += w * 0.2;
  b.spine.rotation.x += w * sw(s, -0.15, 0.1);
  armsOut(b, w, 0.9);
}

function chest(b: Bones, s: number, w: number): void {
  b.spine.rotation.x += w * sw(s, -0.4, 0.25);
  b.chest.rotation.x += w * sw(s, -0.2, 0.15);
  b.upperArmL.rotation.x += w * 0.4;
  b.upperArmR.rotation.x += w * 0.4;
  armsOut(b, w, 1.2);
  b.legL.rotation.x += w * 0.3;
  b.legR.rotation.x += w * 0.3;
  b.upLegL.rotation.x -= w * 0.15;
  b.upLegR.rotation.x -= w * 0.15;
}

function shoulder(b: Bones, s: number, w: number): void {
  b.spine.rotation.z += w * sw(s, 0.3, -0.35);
  b.spine.rotation.y += w * sw(s, -0.5, 0.45);
  b.shoulderR.rotation.z -= w * 0.3;
  b.upperArmL.rotation.z += w * 0.7;
  b.upperArmR.rotation.x += w * 0.3;
  b.legL.rotation.x += w * 0.25;
}

function head(b: Bones, s: number, w: number): void {
  b.spine.rotation.x += w * sw(s, -0.45, 0.35);
  b.neck.rotation.x += w * sw(s, -0.3, 0.45);
  b.upperArmL.rotation.x += w * 0.6;
  b.upperArmR.rotation.x += w * 0.6;
  armsOut(b, w, 0.4);
  b.footL.rotation.x += w * 0.45;
  b.footR.rotation.x += w * 0.45;
}

function bicycle(b: Bones, s: number, w: number): void {
  b.hips.rotation.x += w * sw(s, -1.0, -2.3);
  b.upLegR.rotation.x += w * sw(s, 0.6, -1.4);
  b.legR.rotation.x += w * sw(s, 1.3, 0.1);
  b.upLegL.rotation.x += w * sw(s, -1.3, 0.5);
  b.legL.rotation.x += w * sw(s, 0.2, 0.9);
  armsOut(b, w, 1.3);
}

const POSES: Readonly<Record<StrikePart, (b: Bones, s: number, w: number) => void>> = {
  foot, knee, chest, shoulder, head, bicycle,
};

export function applyStrikePose(b: Bones, part: StrikePart, s: number, w: number): void {
  if (w > 0) POSES[part](b, s, w);
}

/** Strike timeline: impact happens at IMPACT_T of the normalized clip. */
export const IMPACT_T = 0.3;
const IMPACT_SWING = 0.15;
const RECOVER_START = 0.75;
const IK_HALF_WIDTH = 0.18;

/** Swing value for strike time t (fast ease-in swing, eased follow-through). */
export function strikeSwing(t: number): number {
  if (t < IMPACT_T) {
    const u = t / IMPACT_T;
    return -1 + (1 + IMPACT_SWING) * u * u;
  }
  const u = (t - IMPACT_T) / (1 - IMPACT_T);
  return IMPACT_SWING + (1 - IMPACT_SWING) * Math.sin(Math.min(u * 2, 1) * Math.PI * 0.5);
}

/** Pose weight: full until recovery, then fades back to locomotion. */
export function strikeWeight(t: number): number {
  if (t <= 0 || t >= 1) return 0;
  return t < RECOVER_START ? 1 : 1 - (t - RECOVER_START) / (1 - RECOVER_START);
}

/** Bell (0..1..0) centred on impact; drives IK weight and squash/stretch. */
export function impactBell(t: number): number {
  const x = (t - IMPACT_T) / IK_HALF_WIDTH;
  return Math.abs(x) >= 1 ? 0 : 0.5 + 0.5 * Math.cos(x * Math.PI);
}
