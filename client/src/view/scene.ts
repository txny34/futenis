import * as THREE from "three";

// Static scene pieces (lights, ball, ball shadow). Built once.
const BALL_R = 0.11;
const BALL_COLOR = 0xf5e663;
const BALL_EMISSIVE = 0x332e00;
const SPHERE_SEGMENTS = 24;
const SPHERE_RINGS = 16;
const SKY_COLOR = 0xdbe9ff;
const GROUND_COLOR = 0x223344;
const HEMI_INTENSITY = 0.8;
const SUN_COLOR = 0xffffff;
const SUN_INTENSITY = 1.6;
const SUN_POS = { x: -6, y: 14, z: 8 } as const;
const SHADOW_MAP_SIZE = 2048;
const SHADOW_HALF_X = 12;
const SHADOW_HALF_Z = 8;
const SHADOW_OPACITY = 0.45;
const SHADOW_RADIUS_FACTOR = 1.3;

export function addLights(scene: THREE.Scene): THREE.DirectionalLight {
  scene.add(new THREE.HemisphereLight(SKY_COLOR, GROUND_COLOR, HEMI_INTENSITY));
  const sun = new THREE.DirectionalLight(SUN_COLOR, SUN_INTENSITY);
  sun.position.set(SUN_POS.x, SUN_POS.y, SUN_POS.z);
  sun.castShadow = true;
  sun.shadow.mapSize.set(SHADOW_MAP_SIZE, SHADOW_MAP_SIZE);
  const cam = sun.shadow.camera;
  cam.left = -SHADOW_HALF_X;
  cam.right = SHADOW_HALF_X;
  cam.top = SHADOW_HALF_Z;
  cam.bottom = -SHADOW_HALF_Z;
  scene.add(sun);
  return sun;
}

export function makeBall(scene: THREE.Scene): THREE.Mesh {
  const m = new THREE.Mesh(
    new THREE.SphereGeometry(BALL_R, SPHERE_SEGMENTS, SPHERE_RINGS),
    new THREE.MeshStandardMaterial({ color: BALL_COLOR, emissive: BALL_EMISSIVE }),
  );
  m.castShadow = true;
  scene.add(m);
  return m;
}

/** Blob shadow under the ball: always visible so jumps/lobs stay readable even with shadows off. */
export function makeBallShadow(scene: THREE.Scene): THREE.Mesh {
  const m = new THREE.Mesh(
    new THREE.CircleGeometry(BALL_R * SHADOW_RADIUS_FACTOR, SPHERE_SEGMENTS),
    new THREE.MeshBasicMaterial({ color: 0x000000, transparent: true, opacity: SHADOW_OPACITY, depthWrite: false }),
  );
  m.rotation.x = -Math.PI / 2;
  scene.add(m);
  return m;
}
