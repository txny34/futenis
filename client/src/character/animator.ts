import * as THREE from "three";
import { StrikePart } from "./config";
import { applyContactIk } from "./contact";
import { applyAir, applyIdle, applyLocomotion, applyRest } from "./poses";
import { BROW_REST_Y, Rig, groundSnap, resetRig } from "./rig";
import { Side, createSides, sideFromLocalX } from "./sides";
import { applyStrikePose } from "./strikes";
import { impactBell, liftCurve, strikeSwing, strikeWeight } from "./timing";

// Procedural animator. The output pose is a function of CharacterPose plus
// two clocks (breathing time, gait phase) that only advance with dt — no game
// state, no rules. Zero allocations per update.

export interface StrikePose {
  part: StrikePart;
  /** Normalized strike clip time 0..1 (impact at IMPACT_T). 0 = not swinging. */
  t: number;
  /** World-space contact point from the sim (EvKick.Pos), or null. */
  contact: THREE.Vector3 | null;
  /** Striking side. Omitted: from contact (left if it is on the body's left), else right. */
  side?: Side;
}

export interface CharacterPose {
  /** Horizontal speed normalized 0..1. */
  speed: number;
  sprinting: boolean;
  airborne: boolean;
  /** m/s, + rising. */
  verticalVel: number;
  /** Charge 0..1: wind-up of strike.part (or foot) while strike.t === 0. */
  charge: number;
  strike: StrikePose | null;
  /** Yaw in radians about +Y; 0 faces +Z, PI/2 faces +X. */
  facing: number;
}

const TAU = Math.PI * 2;
const GAIT_BASE_HZ = 1.1;
const GAIT_SPEED_HZ = 0.9;
const GAIT_SPRINT_HZ = 0.35;
const SPRINT_BLEND_RATE = 6;
const CONTACT_SQUASH = 0.08;
const SQUASH_HALF_WIDTH = 0.07;
const AIR_STRETCH_PER_MPS = 0.008;
const AIR_STRETCH_MAX = 0.06;
const MAX_HEAD_LIFT = 0.6;
/** Covers the contact squash and spine flex so the brow chain stays in reach. */
const HEAD_LIFT_MARGIN = 0.08;

const local = new THREE.Vector3();

export interface Animator {
  update(pose: CharacterPose, dt: number): void;
}

export function createAnimator(rig: Rig): Animator {
  let time = 0;
  let phase = 0;
  let sprint = 0;
  const sides = createSides(rig.bones);

  /** Contact in root space (local), or false when there is none. */
  function contactLocal(st: StrikePose | null): boolean {
    if (!st || !st.contact) return false;
    rig.root.updateMatrixWorld(true);
    rig.root.worldToLocal(local.copy(st.contact));
    return true;
  }

  function pickSide(st: StrikePose | null, hasLocal: boolean): Side {
    if (st && st.side) return st.side;
    return hasLocal ? sideFromLocalX(local.x) : "R";
  }

  /** Header above standing brow height: rise/jump so the brow can reach. */
  function headLift(st: StrikePose, hasLocal: boolean): void {
    if (st.part !== "head" || !hasLocal) return;
    const need = THREE.MathUtils.clamp(local.y - BROW_REST_Y + HEAD_LIFT_MARGIN, 0, MAX_HEAD_LIFT);
    rig.bones.hips.position.y += need * liftCurve(st.t);
  }

  function stretch(amount: number): void {
    const xz = 1 / Math.sqrt(1 + amount);
    rig.bones.hips.scale.set(xz, 1 + amount, xz);
    rig.bones.hips.position.y += rig.hipsRestY * amount;
  }

  function baseLayers(pose: CharacterPose, k: number): void {
    const air = pose.airborne ? 1 : 0;
    const speed = THREE.MathUtils.clamp(pose.speed, 0, 1);
    applyRest(rig.bones);
    applyIdle(rig.bones, time, k * (1 - speed) * (1 - air));
    applyLocomotion(rig.bones, phase, k * speed * (1 - air), sprint);
    applyAir(rig.bones, pose.verticalVel, k * air);
  }

  function strikeLayer(pose: CharacterPose, side: Side): void {
    const st = pose.strike;
    if (st && st.t > 0) {
      applyStrikePose(rig.bones, sides[side], st.part, strikeSwing(st.t), strikeWeight(st.t));
      return;
    }
    const charge = THREE.MathUtils.clamp(pose.charge, 0, 1);
    applyStrikePose(rig.bones, sides[side], st ? st.part : "foot", -charge, charge);
  }

  function update(pose: CharacterPose, dt: number): void {
    time += dt;
    const speed = THREE.MathUtils.clamp(pose.speed, 0, 1);
    sprint += ((pose.sprinting ? 1 : 0) - sprint) * Math.min(1, dt * SPRINT_BLEND_RATE);
    phase = (phase + dt * TAU * (GAIT_BASE_HZ + GAIT_SPEED_HZ * speed + GAIT_SPRINT_HZ * sprint)) % TAU;
    resetRig(rig);
    rig.root.rotation.y = pose.facing;
    const st = pose.strike;
    const hasLocal = contactLocal(st);
    const side = pickSide(st, hasLocal);
    const active = st !== null && st.t > 0;
    baseLayers(pose, 1 - (active ? strikeWeight(st.t) : 0));
    strikeLayer(pose, side);
    const squash = active ? impactBell(st.t, SQUASH_HALF_WIDTH) : 0;
    const air = pose.airborne ? Math.min(AIR_STRETCH_MAX, Math.abs(pose.verticalVel) * AIR_STRETCH_PER_MPS) : 0;
    stretch(air - CONTACT_SQUASH * squash);
    if (!pose.airborne) groundSnap(rig);
    if (!active || !st.contact) return;
    headLift(st, hasLocal);
    applyContactIk(rig, sides[side], st.part, st.contact, impactBell(st.t));
  }

  return { update };
}
