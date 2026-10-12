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

/** BAJO (instep drive): heel cocked behind with the torso over the ball, the
 * planted knee loaded, arms counter-balancing on the diagonal (far arm forward
 * and across, kick-side arm back); a whipped swing to a ~75° finish. */
function low(b: Bones, x: SideRig, s: number, w: number): void {
  x.kUp.rotation.x += w * (-0.25 + sw(s, 0.95, -1.05));
  x.kLeg.rotation.x += w * (0.35 + sw(s, 1.35, -0.25));
  x.kUp.rotation.y -= x.d * w * 0.2; // open the hip a little: instep, not toe
  x.kFoot.rotation.x += w * 0.45; // pointed toe
  plant(x, w, 0.7);
  coil(b, x, s, w, 0.55, 0.45);
  b.spine.rotation.z -= x.d * w * 0.18; // lean over the planted leg
  b.spine.rotation.x += w * (0.15 - Math.max(0, s) * 0.3); // over the ball, then a small lean back
  x.pArm.rotation.x -= w * (0.75 + sw(s, -0.2, 0.25)); // far arm forward and up
  x.pArm.rotation.z += x.d * w * 0.45;
  x.pFore.rotation.x -= w * 0.75;
  x.kArm.rotation.x += w * (0.45 + sw(s, 0.15, 0.1)); // kick-side arm back for balance
  x.kArm.rotation.z -= x.d * w * 0.65;
  x.kFore.rotation.x -= w * 0.5;
}

/** MEDIO (chest control): arms low, open and back with soft elbows; arch to
 * cushion, then push the chest through. */
function mid(b: Bones, x: SideRig, s: number, w: number): void {
  b.spine.rotation.x += w * sw(s, -0.5, 0.35);
  b.chest.rotation.x += w * sw(s, -0.3, 0.2);
  b.neck.rotation.x += w * sw(s, 0.35, -0.1);
  b.spine.rotation.y += x.d * w * sw(s, -0.25, 0.25);
  armsOut(x, w, 0.75, 0.75);
  x.kArm.rotation.x += w * 0.35;
  x.pArm.rotation.x += w * 0.35;
  x.kFore.rotation.x -= w * 0.85;
  x.pFore.rotation.x -= w * 0.85;
  plant(x, w, 0.55);
  x.kUp.rotation.x -= w * (0.2 + sw(s, 0, 0.15));
  x.kLeg.rotation.x += w * 0.45;
}

/** ALTO (header): crouch with the arms swung back, jump driving the arms up,
 * arch the back and whip the neck through the ball; legs tuck in the air. */
function high(b: Bones, x: SideRig, s: number, w: number): void {
  b.spine.rotation.x += w * sw(s, -0.35, 0.45);
  b.chest.rotation.x += w * sw(s, -0.25, 0.25);
  b.neck.rotation.x += w * sw(s, -0.55, 0.9);
  b.head.rotation.x += w * sw(s, -0.25, 0.35);
  const arms = sw(s, 0.7, -1.25); // back on the load, up and forward on the jump
  x.kArm.rotation.x += w * arms;
  x.pArm.rotation.x += w * arms;
  armsOut(x, w, 0.4, 0.4);
  x.kFore.rotation.x -= w * 0.9;
  x.pFore.rotation.x -= w * 0.9;
  const thighs = sw(s, 0.55, 0.45);
  const knees = 0.3 + sw(s, 0.9, 0.9); // never straight-legged: loaded crouch, bent at contact, tucked after
  x.kUp.rotation.x -= w * thighs;
  x.pUp.rotation.x -= w * thighs;
  x.kLeg.rotation.x += w * knees;
  x.pLeg.rotation.x += w * knees;
  x.kFoot.rotation.x += w * 0.5;
  x.pFoot.rotation.x += w * 0.5;
}

type PoseFn = (b: Bones, x: SideRig, s: number, w: number) => void;
const POSES: Readonly<Record<StrikeTier, PoseFn>> = { low, mid, high };

/** power >= 1 stretches the swing extremes (perfect hits); impact stays put. */
export function applyStrikePose(b: Bones, x: SideRig, tier: StrikeTier, s: number, w: number, power = 1): void {
  if (w > 0) POSES[tier](b, x, s * power, w);
}
export { IMPACT_T } from "./timing";
