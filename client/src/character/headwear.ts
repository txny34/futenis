import * as THREE from "three";
import { BODY } from "./config";
import { BROW_Y, HEAD_SCALE_XZ, HEAD_SCALE_Y, HEAD_Y } from "./head";
import * as S from "./shapes";

// Headwear meshes (built per style change, never per frame), in head-dress
// space. The head is a sphere of radius R widened by HEAD_SCALE_XZ, so a ring
// at height y hugs a circle of radius ringR(y) and the face is +Z.

const R = BODY.headRadius;
const VISOR_R = 0.11;
const VISOR_TILT = 0.12; // the brim dips a little toward the eyes
const BAND_Y = BROW_Y + 0.045; // headband / beanie cuff: on the forehead
const BRIM_Y = BROW_Y + 0.022; // the cap's brim shades the brows

export type Put = (g: THREE.Object3D, geo: THREE.BufferGeometry, mat: THREE.Material, x: number, y: number, z: number, sx?: number, sy?: number, sz?: number) => THREE.Mesh;

/** Radius of the head's horizontal section at height y (before the XZ widening). */
function ringR(y: number): number {
  const dy = (y - HEAD_Y) / HEAD_SCALE_Y;
  return Math.sqrt(Math.max(0, R * R - dy * dy));
}

/** A horizontal ring hugging the head at y (headband, beanie cuff). */
function ring(put: Put, g: THREE.Object3D, mat: THREE.Material, y: number, tube: number, grow = 1.02): THREE.Mesh {
  const band = put(g, new THREE.TorusGeometry(ringR(y) * grow + tube * 0.5, tube, 8, 28), mat, 0, y, 0, HEAD_SCALE_XZ, HEAD_SCALE_XZ, 1.6);
  band.rotation.x = Math.PI / 2;
  return band;
}

/** Crown dome over the top of the head down to `coverage` (0..1 of the sphere). */
function crown(put: Put, g: THREE.Object3D, mat: THREE.Material, grow: number, coverage: number, tilt: number): THREE.Mesh {
  const dome = put(g, S.cap(R * grow, coverage), mat, 0, HEAD_Y, 0, HEAD_SCALE_XZ, HEAD_SCALE_Y, HEAD_SCALE_XZ);
  dome.rotation.x = tilt;
  return dome;
}

/** Baseball cap; backwards turns the brim to the nape. */
export function cap(put: Put, g: THREE.Object3D, mat: THREE.Material, backwards: boolean): void {
  crown(put, g, mat, 1.06, 0.5, backwards ? 0.2 : -0.2); // tilted so it reaches the brim at the front
  const dir = backwards ? -1 : 1;
  const z = ringR(BRIM_Y) * HEAD_SCALE_XZ * 0.9 * dir;
  // A half disc whose straight edge runs along the forehead (CylinderGeometry
  // puts theta 0 on +Z: -pi/2..pi/2 is the front half), stretched forward.
  const brim = put(g, new THREE.CylinderGeometry(VISOR_R, VISOR_R, 0.012, 16, 1, false, -Math.PI / 2, Math.PI), mat, 0, BRIM_Y, z, 1.2, 1, 1.05);
  brim.rotation.set(VISOR_TILT * dir, backwards ? Math.PI : 0, 0);
  put(g, S.ball(0.016, 8, 6), mat, 0, HEAD_Y + R * 1.05, 0); // the button on top
}

export function headband(put: Put, g: THREE.Object3D, mat: THREE.Material): void {
  ring(put, g, mat, BAND_Y, 0.017);
}

/** Wool beanie: a deep crown, a folded cuff over the brows and a pompom. */
export function beanie(put: Put, g: THREE.Object3D, mat: THREE.Material): void {
  crown(put, g, mat, 1.07, 0.56, -0.05);
  ring(put, g, mat, BAND_Y + 0.01, 0.024, 1.05);
  put(g, S.ball(0.04, 10, 8), mat, 0, HEAD_Y + R * 1.12, -0.01);
}
