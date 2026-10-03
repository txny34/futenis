import { StrikeTier } from "./config";
import { Bones } from "./rig";
import { SideRig } from "./sides";

// Three visual strike poses (sim part -> tier via PART_TIER), written for the
// kick side `x` and mirrored by x.d. `s` is the swing: -1 full wind-up,
// 0 impact, +1 full follow-through. `w` blends the whole pose. Arm "out":
// right arm -z, left arm +z, so the kick-side arm goes out with -d.

/** Perfect hits push wind-up and follow-through this much further. */
export const PERFECT_POWER = 1.25;

function sw(s: number, wind: number, follow: number): number {
  return s < 0 ? -s * wind : s * follow;
}

function armsOut(x: SideRig, w: number, kick: number, plant: number): void {
  x.kArm.rotation.z -= x.d * w * kick;
  x.pArm.rotation.z += x.d * w * plant;
}

/** Support leg: loaded knee, heel up, weight on the ball of the foot. */
function plant(x: SideRig, w: number, bend: number): void {
  x.pUp.rotation.x -= w * bend * 0.6;
  x.pLeg.rotation.x += w * bend;
  x.pFoot.rotation.x += w * 0.2;
}

/** Hip vs shoulder counter-rotation (coiled on wind-up, uncoiled after). */
function coil(b: Bones, x: SideRig, s: number, w: number, hip: number, shoulder: number): void {
  b.hips.rotation.y += x.d * w * sw(s, hip, -hip * 0.8);
  b.chest.rotation.y += x.d * w * sw(s, -shoulder, shoulder);
}

/** BAJO: thigh cocked far back with a tight knee, whip through, high finish. */
function low(b: Bones, x: SideRig, s: number, w: number): void {
  x.kUp.rotation.x += w * (-0.6 + sw(s, 1.9, -1.5));
  x.kLeg.rotation.x += w * (0.25 + sw(s, 2.1, -0.2));
  x.kUp.rotation.y -= x.d * w * 0.3; // side-foot: open the hip
  x.kFoot.rotation.x += w * 0.5;
  plant(x, w, 0.8);
  coil(b, x, s, w, 0.6, 0.5);
  b.spine.rotation.z -= x.d * w * 0.25;
  b.spine.rotation.x -= w * (0.1 + Math.max(0, s) * 0.35);
  armsOut(x, w, 0.8, 1.5);
  x.pArm.rotation.x -= w * 0.4;
  x.kArm.rotation.x += w * sw(s, -0.6, 0.8);
  x.pFore.rotation.x -= w * 0.4;
}

/** MEDIO: arch back with arms wide, pop the chest through, fold over after. */
function mid(b: Bones, x: SideRig, s: number, w: number): void {
  b.spine.rotation.x += w * sw(s, -0.75, 0.5);
  b.chest.rotation.x += w * sw(s, -0.4, 0.25);
  b.neck.rotation.x += w * sw(s, 0.4, -0.15);
  b.spine.rotation.y += x.d * w * sw(s, -0.35, 0.35);
  x.kArm.rotation.x += w * sw(s, -0.3, 0.5);
  x.pArm.rotation.x += w * sw(s, -0.3, 0.5);
  armsOut(x, w, 1.4 + sw(s, 0.2, -0.4), 1.4 + sw(s, 0.2, -0.4));
  x.kFore.rotation.x -= w * 0.5;
  x.pFore.rotation.x -= w * 0.5;
  plant(x, w, 0.55);
  x.kUp.rotation.x -= w * (0.25 + sw(s, 0, 0.2));
  x.kLeg.rotation.x += w * 0.5;
}

/** ALTO: crouch-load with the neck cocked back, rise, snap the brow through. */
function high(b: Bones, x: SideRig, s: number, w: number): void {
  b.spine.rotation.x += w * sw(s, -0.65, 0.5);
  b.chest.rotation.x += w * sw(s, -0.3, 0.25);
  b.neck.rotation.x += w * sw(s, -0.6, 1.0);
  b.head.rotation.x += w * sw(s, -0.25, 0.4);
  x.kArm.rotation.x += w * sw(s, 0.6, 1.3);
  x.pArm.rotation.x += w * sw(s, 0.6, 1.3);
  armsOut(x, w, 0.6, 0.6);
  x.kFore.rotation.x -= w * 0.6;
  x.pFore.rotation.x -= w * 0.6;
  const crouch = sw(s, 0.45, 0);
  const knees = sw(s, 0.85, 0.1);
  x.kUp.rotation.x -= w * crouch;
  x.pUp.rotation.x -= w * crouch;
  x.kLeg.rotation.x += w * knees;
  x.pLeg.rotation.x += w * knees;
  x.kFoot.rotation.x += w * 0.6;
  x.pFoot.rotation.x += w * 0.6;
}

type PoseFn = (b: Bones, x: SideRig, s: number, w: number) => void;
const POSES: Readonly<Record<StrikeTier, PoseFn>> = { low, mid, high };

/** power >= 1 stretches the swing extremes (perfect hits); impact stays put. */
export function applyStrikePose(b: Bones, x: SideRig, tier: StrikeTier, s: number, w: number, power = 1): void {
  if (w > 0) POSES[tier](b, x, s * power, w);
}
export { IMPACT_T } from "./timing";
