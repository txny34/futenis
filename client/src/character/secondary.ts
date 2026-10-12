import type * as THREE from "three";

// Secondary motion (visual only, the "janky" charm without a ragdoll): the
// head bobbles on a spring driven by the body's acceleration — leaning
// forward when braking, back when sprinting off, sideways in turns — and the
// hair swings a beat later on a looser spring. Nothing here feeds the sim or
// the hitbox. Zero allocations per update.

const HEAD_K = 140; // spring stiffness (1/s^2)
const HEAD_DAMP = 9; // damping (1/s): a little under critical, so it wobbles
const HAIR_K = 70;
const HAIR_DAMP = 6;
const HEAD_GAIN = 0.012; // rad per m/s^2 of acceleration
const HAIR_GAIN = 0.022;
const MAX_TILT = 0.35;
const MAX_HAIR_SWING = 0.1; // relative to the head: more would lift the hair off the skull
const MAX_ACCEL = 40; // m/s^2: teleports (respawn, replay jumps) are ignored above this

interface Spring { x: number; v: number }

function step(s: Spring, target: number, k: number, damp: number, dt: number, max: number): void {
  s.v += ((target - s.x) * k - s.v * damp) * dt;
  s.x += s.v * dt;
  if (s.x > max) s.x = max;
  else if (s.x < -max) s.x = -max;
}

export interface Secondary {
  update(root: THREE.Object3D, facing: number, dt: number): void;
}

export function createSecondary(head: THREE.Object3D, hair: () => THREE.Object3D): Secondary {
  const headPitch: Spring = { x: 0, v: 0 };
  const headRoll: Spring = { x: 0, v: 0 };
  const hairPitch: Spring = { x: 0, v: 0 };
  const hairRoll: Spring = { x: 0, v: 0 };
  let px = 0, pz = 0, vx = 0, vz = 0, primed = 0;

  return {
    update(root, facing, dt) {
      if (dt <= 0) return;
      const x = root.position.x, z = root.position.z;
      const nvx = (x - px) / dt, nvz = (z - pz) / dt;
      let ax = (nvx - vx) / dt, az = (nvz - vz) / dt;
      px = x; pz = z; vx = nvx; vz = nvz;
      if (primed < 2 || Math.hypot(ax, az) > MAX_ACCEL) { // first frames / a teleport: no kick
        primed++;
        ax = 0;
        az = 0;
      }
      // Into the body frame: forward = facing (+Z at 0), side = its right.
      const sin = Math.sin(facing), cos = Math.cos(facing);
      const fwd = ax * sin + az * cos;
      const side = ax * cos - az * sin;
      step(headPitch, fwd * HEAD_GAIN, HEAD_K, HEAD_DAMP, dt, MAX_TILT); // braking (fwd < 0) pitches the head forward
      step(headRoll, -side * HEAD_GAIN, HEAD_K, HEAD_DAMP, dt, MAX_TILT);
      step(hairPitch, fwd * HAIR_GAIN, HAIR_K, HAIR_DAMP, dt, MAX_HAIR_SWING);
      step(hairRoll, -side * HAIR_GAIN, HAIR_K, HAIR_DAMP, dt, MAX_HAIR_SWING);
      head.rotation.x = -headPitch.x;
      head.rotation.z = headRoll.x;
      const h = hair();
      h.rotation.x = -hairPitch.x;
      h.rotation.z = hairRoll.x;
    },
  };
}
