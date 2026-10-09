import * as THREE from "three";
import { THEME } from "../env/theme";
import { CharacterLook } from "./config";

// Toon palette: one shared 3-step gradient, one material per look slot.

const GRADIENT_STEPS = new Uint8Array([90, 170, 255]);
const EYE_WHITE = THEME.fx.eyeWhite;
const EYE_PUPIL = THEME.fx.eyePupil;

let gradient: THREE.DataTexture | null = null;

function gradientMap(): THREE.DataTexture {
  if (gradient) return gradient;
  const tex = new THREE.DataTexture(GRADIENT_STEPS, GRADIENT_STEPS.length, 1, THREE.RedFormat);
  tex.minFilter = THREE.NearestFilter;
  tex.magFilter = THREE.NearestFilter;
  tex.generateMipmaps = false;
  tex.needsUpdate = true;
  gradient = tex;
  return tex;
}

function toon(color: number): THREE.MeshToonMaterial {
  return new THREE.MeshToonMaterial({ color, gradientMap: gradientMap() });
}

export interface LookMaterials {
  readonly shirt: THREE.MeshToonMaterial;
  readonly shorts: THREE.MeshToonMaterial;
  readonly skin: THREE.MeshToonMaterial;
  readonly hair: THREE.MeshToonMaterial;
  readonly boots: THREE.MeshToonMaterial;
  readonly eyeWhite: THREE.MeshBasicMaterial;
  readonly pupil: THREE.MeshBasicMaterial;
}

export function createMaterials(look: CharacterLook): LookMaterials {
  return {
    shirt: toon(look.shirt),
    shorts: toon(look.shorts),
    skin: toon(look.skin),
    hair: toon(look.hair),
    boots: toon(look.boots),
    eyeWhite: new THREE.MeshBasicMaterial({ color: EYE_WHITE }),
    pupil: new THREE.MeshBasicMaterial({ color: EYE_PUPIL }),
  };
}

/** Recolors in place (team swap / skin preview) without rebuilding geometry. */
export function applyLook(m: LookMaterials, look: CharacterLook): void {
  m.shirt.color.setHex(look.shirt);
  m.shorts.color.setHex(look.shorts);
  m.skin.color.setHex(look.skin);
  m.hair.color.setHex(look.hair);
  m.boots.color.setHex(look.boots);
}

export function disposeMaterials(m: LookMaterials): void {
  for (const mat of Object.values(m)) mat.dispose();
}
