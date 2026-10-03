import { StrikePart } from "./config";
import { Bones } from "./rig";
import { SideRig } from "./sides";

// Per-part strike poses, written for the kick side `x` and mirrored by x.d.
// `s` is the swing: -1 full wind-up, 0 impact, +1 full follow-through.
// `w` blends the whole pose. Arm "out": right arm -z, left arm +z, so the
// kick-side arm goes out with -d and the plant-side arm with +d.

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

function foot(b: Bones, x: SideRig, s: number, w: number): void {
  x.kUp.rotation.x += w * (-0.7 + sw(s, 1.7, -1.4));
  x.kLeg.rotation.x += w * (0.35 + sw(s, 1.95, -0.3));
  x.kFoot.rotation.x += w * 0.5;
  plant(x, w, 0.75);
  coil(b, x, s, w, 0.5, 0.45);
  b.spine.rotation.z -= x.d * w * 0.22;
  b.spine.rotation.x -= w * (0.12 + Math.max(0, s) * 0.3);
  armsOut(x, w, 0.7, 1.4);
  x.pArm.rotation.x -= w * 0.4;
  x.kArm.rotation.x += w * sw(s, -0.5, 0.7);
  x.pFore.rotation.x -= w * 0.4;
}

function knee(b: Bones, x: SideRig, s: number, w: number): void {
  x.kUp.rotation.x += w * sw(s, 0.6, -2.0);
  x.kLeg.rotation.x += w * sw(s, 0.6, 2.3);
  x.kFoot.rotation.x += w * 0.4;
  plant(x, w, 0.5);
  coil(b, x, s, w, 0.3, 0.3);
  b.spine.rotation.x += w * sw(s, -0.3, 0.4);
  armsOut(x, w, 1.1, 1.1);
  x.kFore.rotation.x -= w * 0.7;
  x.pFore.rotation.x -= w * 0.7;
}

function chest(b: Bones, x: SideRig, s: number, w: number): void {
  b.spine.rotation.x += w * sw(s, -0.65, 0.35);
  b.chest.rotation.x += w * sw(s, -0.35, 0.2);
  b.neck.rotation.x += w * sw(s, 0.35, -0.1);
  x.kArm.rotation.x += w * 0.7;
  x.pArm.rotation.x += w * 0.7;
  armsOut(x, w, 1.4, 1.4);
  plant(x, w, 0.5);
  x.kUp.rotation.x -= w * 0.3;
  x.kLeg.rotation.x += w * 0.5;
}

function shoulder(b: Bones, x: SideRig, s: number, w: number): void {
  b.spine.rotation.z += x.d * w * sw(s, 0.45, -0.5);
  b.spine.rotation.y += x.d * w * sw(s, -0.7, 0.6);
  b.chest.rotation.y += x.d * w * sw(s, -0.3, 0.3);
  x.kShoulder.rotation.z -= x.d * w * 0.45;
  armsOut(x, w, 0.5, 1.0);
  x.kArm.rotation.x += w * 0.6;
  plant(x, w, 0.5);
  x.kLeg.rotation.x += w * 0.3;
}

function head(b: Bones, x: SideRig, s: number, w: number): void {
  b.spine.rotation.x += w * sw(s, -0.6, 0.45);
  b.chest.rotation.x += w * sw(s, -0.25, 0.2);
  b.neck.rotation.x += w * sw(s, -0.5, 0.9);
  b.head.rotation.x += w * sw(s, -0.2, 0.35);
  x.kArm.rotation.x += w * sw(s, 0.5, 1.2);
  x.pArm.rotation.x += w * sw(s, 0.5, 1.2);
  armsOut(x, w, 0.5, 0.5);
  x.kFore.rotation.x -= w * 0.6;
  x.pFore.rotation.x -= w * 0.6;
  const crouch = sw(s, 0.35, 0);
  const knees = sw(s, 0.7, 0.05);
  x.kUp.rotation.x -= w * crouch;
  x.pUp.rotation.x -= w * crouch;
  x.kLeg.rotation.x += w * knees;
  x.pLeg.rotation.x += w * knees;
  x.kFoot.rotation.x += w * 0.6;
  x.pFoot.rotation.x += w * 0.6;
}

/**
 * Bicycle: pelvis rolls backward about the lateral (X) axis until the torso
 * is horizontal facing up at impact (hips.x ~ -PI/2). The support leg swings
 * up first, then the kick leg scissors over it to the ball.
 */
function bicycle(b: Bones, x: SideRig, s: number, w: number): void {
  b.hips.rotation.x += w * (-1.55 + sw(s, 0.95, -0.35));
  x.pUp.rotation.x += w * (-1.4 + sw(s, 1.4, 0.9));
  x.pLeg.rotation.x += w * (0.3 + sw(s, 0, 0.9));
  x.kUp.rotation.x += w * (-1.6 + sw(s, 1.9, -0.6));
  x.kLeg.rotation.x += w * (0.1 + sw(s, 1.6, 0));
  x.kFoot.rotation.x += w * 0.4;
  b.neck.rotation.x += w * 0.45;
  armsOut(x, w, 1.4, 1.4);
  x.kFore.rotation.x -= w * 0.5;
  x.pFore.rotation.x -= w * 0.5;
}

type PoseFn = (b: Bones, x: SideRig, s: number, w: number) => void;
const POSES: Readonly<Record<StrikePart, PoseFn>> = { foot, knee, chest, shoulder, head, bicycle };

export function applyStrikePose(b: Bones, x: SideRig, part: StrikePart, s: number, w: number): void {
  if (w > 0) POSES[part](b, x, s, w);
}
export { IMPACT_T } from "./timing";
