import * as THREE from "three";
import { StrikePart } from "./config";
import { applyContactIk } from "./contact";
import { applyAir, applyIdle, applyLocomotion } from "./poses";
import { Rig, resetRig } from "./rig";
import { applyStrikePose, impactBell, strikeSwing, strikeWeight } from "./strikes";

// Procedural animator. The output pose is a function of CharacterPose plus
// two clocks (breathing time, gait phase) that only advance with dt — no game
// state, no rules. Zero allocations per update.

export interface StrikePose {
  part: StrikePart;
  /** Normalized strike clip time 0..1 (impact at IMPACT_T). 0 = not swinging. */
  t: number;
  /** World-space contact point from the sim (EvKick.Pos), or null. */
  contact: THREE.Vector3 | null;
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
const STRIKE_STRETCH = 0.07;
const AIR_STRETCH_PER_MPS = 0.008;
const AIR_STRETCH_MAX = 0.06;

export interface Animator {
  update(pose: CharacterPose, dt: number): void;
}

export function createAnimator(rig: Rig): Animator {
  let time = 0;
  let phase = 0;
  let sprint = 0;

  function stretch(amount: number): void {
    const xz = 1 / Math.sqrt(1 + amount);
    rig.bones.hips.scale.set(xz, 1 + amount, xz);
    rig.bones.hips.position.y += rig.hipsRestY * amount;
  }

  function baseLayers(pose: CharacterPose, k: number): void {
    const air = pose.airborne ? 1 : 0;
    const speed = THREE.MathUtils.clamp(pose.speed, 0, 1);
    applyIdle(rig.bones, time, k * (1 - speed) * (1 - air));
    applyLocomotion(rig.bones, phase, k * speed * (1 - air), sprint);
    applyAir(rig.bones, pose.verticalVel, k * air);
  }

  function strikeLayer(pose: CharacterPose): void {
    const st = pose.strike;
    if (st && st.t > 0) {
      applyStrikePose(rig.bones, st.part, strikeSwing(st.t), strikeWeight(st.t));
      return;
    }
    const charge = THREE.MathUtils.clamp(pose.charge, 0, 1);
    applyStrikePose(rig.bones, st ? st.part : "foot", -charge, charge);
  }

  function update(pose: CharacterPose, dt: number): void {
    time += dt;
    const speed = THREE.MathUtils.clamp(pose.speed, 0, 1);
    sprint += ((pose.sprinting ? 1 : 0) - sprint) * Math.min(1, dt * SPRINT_BLEND_RATE);
    phase = (phase + dt * TAU * (GAIT_BASE_HZ + GAIT_SPEED_HZ * speed + GAIT_SPRINT_HZ * sprint)) % TAU;
    resetRig(rig);
    rig.root.rotation.y = pose.facing;
    const st = pose.strike;
    const sw = st && st.t > 0 ? strikeWeight(st.t) : 0;
    baseLayers(pose, 1 - sw);
    strikeLayer(pose);
    const bell = st && st.t > 0 ? impactBell(st.t) : 0;
    const air = pose.airborne ? Math.min(AIR_STRETCH_MAX, Math.abs(pose.verticalVel) * AIR_STRETCH_PER_MPS) : 0;
    stretch(STRIKE_STRETCH * bell + air);
    if (st && st.contact && bell > 0) applyContactIk(rig, st.part, st.contact, bell);
  }

  return { update };
}
