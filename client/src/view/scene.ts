import * as THREE from "three";

// Static scene pieces (lights). Built once.
const SKY_COLOR = 0xdbe9ff;
const GROUND_COLOR = 0x223344;
const HEMI_INTENSITY = 0.8;
const SUN_COLOR = 0xffffff;
const SUN_INTENSITY = 1.6;
const SUN_POS = { x: -6, y: 14, z: 8 } as const;
const SHADOW_MAP_SIZE = 2048;
const SHADOW_HALF_X = 12;
const SHADOW_HALF_Z = 8;

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
