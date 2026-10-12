import * as THREE from "three";
import { StrikeTier } from "./config";
import { applyContactIk } from "./contact";
import { applyCheer, applySlump, cheerHop } from "./gestures";
import { applyAir, applyIdle, applyLocomotion, applyRest } from "./poses";
import { BROW_REST_Y, Rig, groundSnap, resetRig } from "./rig";
import { Side, createSides, sideFromLocalX } from "./sides";
import { PERFECT_POWER, applyStrikePose } from "./strikes";
import { impactBell, liftCurve, strikeSwing, strikeWeight } from "./timing";

// Procedural animator. The output pose is a function of CharacterPose plus
// two clocks (breathing time, gait phase) that only advance with dt — no game
// state, no rules. Zero allocations per update.

export interface StrikePose {
  /** Visual tier (map sim parts with PART_TIER). */
  tier: StrikeTier;
  /** Perfect-quality hit: exaggerated wind-up and follow-through. */
  perfect?: boolean;
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
  airborne: boolean;
  /** m/s, + rising. */
  verticalVel: number;
  /** Charge 0..1: wind-up of strike.tier (or low) while strike.t === 0. */
  charge: number;
  strike: StrikePose | null;
  /** Yaw in radians about +Y; 0 faces +Z, PI/2 faces +X. */
  facing: number;
  /** World point to keep an eye on (the ball), or null/omitted to look ahead. */
  lookAt?: THREE.Vector3 | null;
  /** Reaction weights 0..1 (gestures.ts): festejo after winning a set/match, bajón after losing. */
  cheer?: number;
  slump?: number;
}

const TAU = Math.PI * 2;
const GAIT_BASE_HZ = 1.1;
const GAIT_SPEED_HZ = 0.9;
const CONTACT_SQUASH = 0.08;
const SQUASH_HALF_WIDTH = 0.07;
const AIR_STRETCH_PER_MPS = 0.008;
const AIR_STRETCH_MAX = 0.06;
const MAX_HEAD_LIFT = 0.6;
/** Headers always hop a little so ALTO reads as a jump. */
const MIN_HEAD_HOP = 0.08;
const CHARGE_RUN_DAMP = 0.75;
const LAND_SQUASH = 0.09; // touchdown absorb after a jump
const LAND_S = 0.18;
/** Covers the contact squash and spine flex so the brow chain stays in reach. */
const HEAD_LIFT_MARGIN = 0.14; // + the header's bent knees lowering the brow at contact

const local = new THREE.Vector3();

export interface Animator {
  update(pose: CharacterPose, dt: number): void;
}

export function createAnimator(rig: Rig): Animator {
  let time = 0;
  let phase = 0;
  let wasAir = false;
  let land = 0; // 1 at touchdown, fades to 0 over LAND_S
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

  /** Header: hop, and rise further when the ball is above standing brow height. */
  function headLift(st: StrikePose, hasLocal: boolean): void {
    if (st.tier !== "high") return;
    const reach = hasLocal ? local.y - BROW_REST_Y + HEAD_LIFT_MARGIN : 0;
    const need = THREE.MathUtils.clamp(reach, MIN_HEAD_HOP, MAX_HEAD_LIFT);
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
    const gesture = Math.max(pose.cheer ?? 0, pose.slump ?? 0); // a reaction replaces the ready stance
    applyIdle(rig.bones, time, k * (1 - speed) * (1 - air) * (1 - gesture));
    // A charged kick takes over the legs: the run fades so the cocked heel never folds through the thigh.
    const charge = THREE.MathUtils.clamp(pose.charge, 0, 1);
    applyLocomotion(rig.bones, phase, k * speed * (1 - air) * (1 - CHARGE_RUN_DAMP * charge));
    applyAir(rig.bones, pose.verticalVel, k * air);
    applyCheer(rig.bones, time, k * (pose.cheer ?? 0)); // not gated by air: no snap when jumping mid-cheer
    applySlump(rig.bones, time, k * (pose.slump ?? 0));
  }

  function strikeLayer(pose: CharacterPose, side: Side): void {
    const st = pose.strike;
    if (st && st.t > 0) {
      const power = st.perfect ? PERFECT_POWER : 1;
      applyStrikePose(rig.bones, sides[side], st.tier, strikeSwing(st.t), strikeWeight(st.t), power);
      return;
    }
    const charge = THREE.MathUtils.clamp(pose.charge, 0, 1);
    applyStrikePose(rig.bones, sides[side], st ? st.tier : "low", -charge, charge);
  }

  function update(pose: CharacterPose, dt: number): void {
    time += dt;
    const speed = THREE.MathUtils.clamp(pose.speed, 0, 1);
    phase = (phase + dt * TAU * (GAIT_BASE_HZ + GAIT_SPEED_HZ * speed)) % TAU;
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
    if (wasAir && !pose.airborne) land = 1;
    wasAir = pose.airborne;
    land = Math.max(0, land - dt / LAND_S);
    stretch(air - CONTACT_SQUASH * squash - LAND_SQUASH * land * land);
    if (!pose.airborne) {
      groundSnap(rig);
      rig.bones.hips.position.y += cheerHop(time, pose.cheer ?? 0);
    }
    if (!active) return;
    headLift(st, hasLocal);
    if (st.contact) applyContactIk(rig, sides[side], st.tier, st.contact, impactBell(st.t));
  }

  return { update };
}
