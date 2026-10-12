import * as THREE from "three";
import { THEME } from "../env/theme";
import { CharacterLook } from "./config";

// Toon palette: one shared 4-step gradient, one material per look slot, and a
// fresnel rim light (the arena's neon glow on the silhouette) patched into the
// toon shader — one shared program, no extra draw calls.

const GRADIENT_STEPS = new Uint8Array([105, 165, 215, 255]);
const RIM_POWER = "2.6";
const RIM_STRENGTH = "0.55";
const RIM = new THREE.Color(THEME.fx.rim);
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

function withRim(mat: THREE.MeshToonMaterial): THREE.MeshToonMaterial {
  mat.onBeforeCompile = (shader) => {
    shader.uniforms.rimColor = { value: RIM };
    shader.fragmentShader = `uniform vec3 rimColor;\n${shader.fragmentShader}`.replace(
      "#include <dithering_fragment>",
      `float rim = pow(1.0 - clamp(dot(normal, normalize(vViewPosition)), 0.0, 1.0), ${RIM_POWER});
      gl_FragColor.rgb += rimColor * rim * ${RIM_STRENGTH};
      #include <dithering_fragment>`,
    );
  };
  mat.customProgramCacheKey = () => "futenis-toon-rim";
  return mat;
}

function toon(color: number): THREE.MeshToonMaterial {
  return withRim(new THREE.MeshToonMaterial({ color, gradientMap: gradientMap() }));
}

export interface LookMaterials {
  readonly shirt: THREE.MeshToonMaterial;
  readonly shorts: THREE.MeshToonMaterial;
  readonly trim: THREE.MeshToonMaterial;
  readonly skin: THREE.MeshToonMaterial;
  readonly hair: THREE.MeshToonMaterial;
  readonly boots: THREE.MeshToonMaterial;
  readonly eyeWhite: THREE.MeshBasicMaterial;
  readonly pupil: THREE.MeshBasicMaterial;
  readonly iris: THREE.MeshBasicMaterial;
  /** Sneaker soles and laces. */
  readonly sole: THREE.MeshToonMaterial;
  /** Mouth decal: lit like the skin, its map set by the face (atlas cell). */
  readonly mouth: THREE.MeshToonMaterial;
  /** Headwear (cap, headband): colored by the style. */
  readonly hat: THREE.MeshToonMaterial;
}

export function createMaterials(look: CharacterLook): LookMaterials {
  return {
    shirt: toon(look.shirt),
    shorts: toon(look.shorts),
    trim: toon(look.trim),
    skin: toon(look.skin),
    hair: toon(look.hair),
    boots: toon(look.boots),
    eyeWhite: new THREE.MeshBasicMaterial({ color: EYE_WHITE }),
    pupil: new THREE.MeshBasicMaterial({ color: EYE_PUPIL }),
    iris: new THREE.MeshBasicMaterial({ color: THEME.fx.iris }),
    sole: toon(THEME.fx.sole),
    mouth: new THREE.MeshToonMaterial({ gradientMap: gradientMap(), alphaTest: 0.5 }),
    hat: toon(THEME.fx.hatColors[0]),
  };
}

/** Recolors in place (team swap / skin preview) without rebuilding geometry. */
export function applyLook(m: LookMaterials, look: CharacterLook): void {
  m.shirt.color.setHex(look.shirt);
  m.shorts.color.setHex(look.shorts);
  m.trim.color.setHex(look.trim);
  m.skin.color.setHex(look.skin);
  m.hair.color.setHex(look.hair);
  m.boots.color.setHex(look.boots);
}

export function disposeMaterials(m: LookMaterials): void {
  m.mouth.map?.dispose(); // the face's view of the shared atlas
  for (const mat of Object.values(m)) mat.dispose();
}
