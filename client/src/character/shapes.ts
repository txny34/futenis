import * as THREE from "three";

// Low-poly generated shapes. All limb shapes hang from their bone origin
// downwards (-Y) so they can be parented directly to a rig bone.

const RADIAL = 10;
const CAP_STEPS = 3;
const HALF_PI = Math.PI / 2;

/** Tapered capsule from y=0 (radius rTop) down to y=-len (radius rBot). */
export function taperCapsule(rTop: number, rBot: number, len: number): THREE.LatheGeometry {
  const pts: THREE.Vector2[] = [];
  for (let i = 0; i <= CAP_STEPS; i++) {
    const a = -HALF_PI + (i / CAP_STEPS) * HALF_PI;
    pts.push(new THREE.Vector2(rBot * Math.cos(a), -len + rBot * Math.sin(a)));
  }
  for (let i = 1; i <= CAP_STEPS; i++) {
    const a = (i / CAP_STEPS) * HALF_PI;
    pts.push(new THREE.Vector2(rTop * Math.cos(a), rTop * Math.sin(a)));
  }
  return new THREE.LatheGeometry(pts, RADIAL);
}

/** Lathe from a [radius, y] profile listed bottom to top. */
export function profileLathe(profile: ReadonlyArray<readonly [number, number]>): THREE.LatheGeometry {
  const pts = profile.map(([r, y]) => new THREE.Vector2(r, y));
  return new THREE.LatheGeometry(pts, RADIAL + 2);
}

export function ball(radius: number, w = 12, h = 8): THREE.SphereGeometry {
  return new THREE.SphereGeometry(radius, w, h);
}

/** Spherical cap (hair) covering the top `coverage` (0..1) of a sphere. */
export function cap(radius: number, coverage: number): THREE.SphereGeometry {
  return new THREE.SphereGeometry(radius, 14, 7, 0, Math.PI * 2, 0, Math.PI * coverage);
}

export function bootShape(radius: number, length: number): THREE.CapsuleGeometry {
  const geo = new THREE.CapsuleGeometry(radius, length, CAP_STEPS, 8);
  geo.rotateX(HALF_PI);
  return geo;
}

export const PELVIS_PROFILE: ReadonlyArray<readonly [number, number]> = [
  [0, -0.12], [0.1, -0.11], [0.155, -0.05], [0.165, 0.02], [0.15, 0.07], [0, 0.08],
];
export const LOWER_TORSO_PROFILE: ReadonlyArray<readonly [number, number]> = [
  [0, -0.1], [0.15, -0.09], [0.155, 0.02], [0.17, 0.14], [0, 0.15],
];
export const UPPER_TORSO_PROFILE: ReadonlyArray<readonly [number, number]> = [
  [0, -0.08], [0.17, -0.08], [0.19, 0.1], [0.2, 0.21], [0.15, 0.28], [0.05, 0.3], [0, 0.3],
];
