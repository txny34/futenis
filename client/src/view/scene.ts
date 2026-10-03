import * as THREE from "three";
import { LAMP_POSITIONS } from "../env/structure";
import { THEME } from "../env/theme";

// Night stadium lighting, built once:
// - dim hemisphere (navy sky / near-black ground) for the night ambience,
// - one overhead key light, the ONLY shadow caster, its shadow camera tight on the court,
// - four tower spotlights (no shadows) aimed at the court for the stadium look.
const HEMI_INTENSITY = 0.55;
const KEY_INTENSITY = 1.9;
// Tilted only along X: the shadow camera's right axis is then world Z, so the
// ortho box lines up with the court (left/right = Z, top/bottom = X).
const KEY_POS = { x: -4, y: 16, z: 0 } as const;
const SHADOW_MAP_SIZE = 2048;
const SHADOW_HALF_X = 10.8; // court is 20 x 10: shadow texels stay on the court
const SHADOW_HALF_Z = 5.8;
const SHADOW_NEAR = 1;
const SHADOW_FAR = 30;
const SHADOW_BIAS = -0.0004;
const SHADOW_NORMAL_BIAS = 0.02;
const SPOT_INTENSITY = 1.1;
const SPOT_ANGLE = Math.PI / 7;
const SPOT_PENUMBRA = 0.6;
const SPOT_DECAY = 0; // constant falloff: predictable at stadium distances
const SPOT_AIM_X = 0.35; // spots cross: each aims at the far half of the court

export function addLights(scene: THREE.Scene): THREE.DirectionalLight {
  scene.add(new THREE.HemisphereLight(THEME.hemiSky, THEME.hemiGround, HEMI_INTENSITY));
  const key = new THREE.DirectionalLight(THEME.lightKey, KEY_INTENSITY);
  key.position.set(KEY_POS.x, KEY_POS.y, KEY_POS.z);
  key.castShadow = true;
  key.shadow.mapSize.set(SHADOW_MAP_SIZE, SHADOW_MAP_SIZE);
  key.shadow.bias = SHADOW_BIAS;
  key.shadow.normalBias = SHADOW_NORMAL_BIAS;
  const cam = key.shadow.camera;
  cam.left = -SHADOW_HALF_Z;
  cam.right = SHADOW_HALF_Z;
  cam.top = SHADOW_HALF_X;
  cam.bottom = -SHADOW_HALF_X;
  cam.near = SHADOW_NEAR;
  cam.far = SHADOW_FAR;
  key.target.position.set(0, 0, 0);
  scene.add(key, key.target);
  for (const [x, y, z] of LAMP_POSITIONS) {
    const spot = new THREE.SpotLight(THEME.lightSpot, SPOT_INTENSITY, 0, SPOT_ANGLE, SPOT_PENUMBRA, SPOT_DECAY);
    spot.position.set(x, y, z);
    spot.target.position.set(-x * SPOT_AIM_X, 0, 0);
    scene.add(spot, spot.target);
  }
  return key;
}
