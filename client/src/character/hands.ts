import * as THREE from "three";
import type { CharacterPose } from "./animator";
import * as S from "./shapes";

// Hands with a pose: a flat palm, a finger block hinged at the knuckles and a
// thumb. `curl` closes them: relaxed at rest, a loose fist when running, open
// in the air and when striking (arms out for balance). Visual only; no
// allocations per update.

const KNUCKLE_Y = -0.078;
const PROXIMAL = 0.03;
const CURL_MAX = 1.55; // rad: fingers folded into the palm
const THUMB_CURL = 0.6;
const CURL_RATE = 10; // 1/s smoothing
const CURL_REST = 0.4;
const CURL_RUN = 0.9;
const CURL_OPEN = 0.12;

export interface HandRig { readonly fingers: THREE.Object3D; readonly tips: THREE.Object3D; readonly thumb: THREE.Object3D; readonly inward: number }

function mesh(parent: THREE.Object3D, geo: THREE.BufferGeometry, mat: THREE.Material, x: number, y: number, z: number, sx: number, sy: number, sz: number): THREE.Mesh {
  const m = new THREE.Mesh(geo, mat);
  m.position.set(x, y, z);
  m.scale.set(sx, sy, sz);
  m.castShadow = true;
  parent.add(m);
  return m;
}

/** Builds a hand on `bone`; inward = the body's side (-1 for the left hand, whose body is at -X). */
export function buildHand(bone: THREE.Object3D, skin: THREE.Material, inward: number): HandRig {
  mesh(bone, S.ball(0.052, 12, 10), skin, 0, -0.045, 0.004, 0.6, 1, 0.95); // palm, thin across the inward axis
  // Four fingers as one soft block in two phalanges, so a curl reads as a round fist.
  const fingers = new THREE.Group();
  fingers.position.set(0, KNUCKLE_Y, 0.004);
  bone.add(fingers);
  mesh(fingers, S.taperCapsule(0.021, 0.02, PROXIMAL), skin, 0, 0, 0, 1.2, 1, 2);
  const tips = new THREE.Group();
  tips.position.y = -PROXIMAL;
  fingers.add(tips);
  mesh(tips, S.taperCapsule(0.02, 0.017, 0.026), skin, 0, 0, 0, 1.2, 1, 1.95);
  const thumb = new THREE.Group();
  thumb.position.set(inward * 0.018, -0.036, 0.032);
  bone.add(thumb);
  mesh(thumb, S.taperCapsule(0.017, 0.013, 0.046), skin, 0, 0, 0, 1, 1, 1).rotation.x = -0.65;
  return { fingers, tips, thumb, inward };
}

/** How closed the hands should be for this pose (0 open .. 1 fist). */
function targetCurl(pose: CharacterPose): number {
  if (pose.airborne || (pose.strike !== null && pose.strike.t > 0)) return CURL_OPEN;
  return CURL_REST + (CURL_RUN - CURL_REST) * Math.min(1, pose.speed * 1.4);
}

export interface Hands { update(pose: CharacterPose, dt: number): void }

export function createHands(hands: readonly HandRig[]): Hands {
  let curl = CURL_REST;
  return {
    update(pose, dt) {
      curl += (targetCurl(pose) - curl) * (1 - Math.exp(-dt * CURL_RATE));
      for (const h of hands) {
        h.fingers.rotation.z = h.inward * curl * CURL_MAX * 0.55; // knuckles, then the tips: a round fist
        h.tips.rotation.z = h.inward * curl * CURL_MAX * 0.6;
        h.thumb.rotation.z = h.inward * curl * THUMB_CURL;
      }
    },
  };
}
