import * as THREE from "three";
import type { CharacterPose } from "./animator";
import { BODY } from "./config";
import { Rig } from "./rig";
import { strikeWeight } from "./timing";

// Head tracking: the player keeps an eye on the ball. Neck and head turn
// toward pose.lookAt within natural limits, smoothed, and give way to the
// strike poses (which aim the head themselves). The face reads the same
// yaw/pitch to move the pupils. Visual only; zero allocations per update.

const MAX_YAW = 1.0; // rad (~57°)
const MAX_PITCH = 0.6;
const RATE = 7; // 1/s: a quick but not robotic glance
const NECK_SHARE = 0.4; // the rest goes to the head
const EYE_HEIGHT = BODY.headCenter;

const local = new THREE.Vector3();
const clamp = (v: number, m: number): number => Math.max(-m, Math.min(m, v));

export interface Look {
  update(pose: CharacterPose, dt: number): void;
  /** Smoothed gaze (rad) for the eyes: + yaw = toward the character's left. */
  readonly yaw: number;
  readonly pitch: number;
}

export function createLook(rig: Rig): Look {
  let yaw = 0;
  let pitch = 0;
  return {
    get yaw() { return yaw; },
    get pitch() { return pitch; },
    update(pose, dt) {
      let ty = 0;
      let tp = 0;
      const st = pose.strike;
      const free = 1 - (st !== null && st.t > 0 ? strikeWeight(st.t) : 0);
      if (pose.lookAt && free > 0) {
        rig.root.updateMatrixWorld();
        rig.root.worldToLocal(local.copy(pose.lookAt));
        ty = clamp(Math.atan2(local.x, local.z), MAX_YAW) * free; // the character faces +Z
        tp = clamp(Math.atan2(local.y - EYE_HEIGHT, Math.hypot(local.x, local.z)), MAX_PITCH) * free;
      }
      const k = 1 - Math.exp(-dt * RATE);
      yaw += (ty - yaw) * k;
      pitch += (tp - pitch) * k;
      const b = rig.bones;
      b.neck.rotation.y += yaw * NECK_SHARE;
      b.head.rotation.y += yaw * (1 - NECK_SHARE);
      b.neck.rotation.x -= pitch * NECK_SHARE; // + x nods down: a ball above lifts the chin
      b.head.rotation.x -= pitch * (1 - NECK_SHARE);
    },
  };
}
