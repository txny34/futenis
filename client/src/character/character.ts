import * as THREE from "three";
import { Animator, CharacterPose, createAnimator } from "./animator";
import { CharacterLook, DEFAULT_LOOK } from "./config";
import { Emotion } from "./expressions";
import { LookMaterials, applyLook, createMaterials, disposeMaterials } from "./materials";
import { TORSO_DEPTH, countTriangles, dressRig, restyle } from "./parts";
import { buildAccessories, restyleAccessories } from "./accessories";
import { Jersey, createJersey } from "./jersey";
import { createSecondary } from "./secondary";
import { createHands } from "./hands";
import { createLook } from "./look";
import { CharacterStyle } from "./style";
import { THEME } from "../env/theme";
import { Rig, createRig } from "./rig";

// Public entry point: a procedural toy-athlete puppet. The caller positions
// `root` (feet at local Y=0) from the interpolated snapshot and calls
// update() once per render frame.

export interface Character {
  readonly root: THREE.Object3D;
  readonly rig: Rig;
  readonly triangles: number;
  update(pose: CharacterPose, dt: number): void;
  /** Shows a facial expression for holdS seconds (game events: points, perfects...). */
  express(emotion: Emotion, holdS: number): void;
  setLook(look: CharacterLook): void;
  /** Hair, headwear, skin/hair/hat colors, head size and build (customizer, bots). */
  setStyle(style: Readonly<CharacterStyle>): void;
  /** Name and number on the back of the shirt, printed in the kit's ink. */
  setJersey(number: number, name: string): void;
  dispose(): void;
}

export function createCharacter(look: CharacterLook = DEFAULT_LOOK): Character {
  const rig = createRig();
  const materials: LookMaterials = createMaterials(look);
  const dressing = dressRig(rig, materials);
  const face = dressing.face;
  const extras = buildAccessories(rig, dressing.headDress, materials);
  const secondary = createSecondary(dressing.headDress, () => dressing.hair);
  const hands = createHands(dressing.hands);
  const gaze = createLook(rig);
  let style: Readonly<CharacterStyle> | null = null;
  const styleColors = (): void => {
    if (style === null) return;
    const fx = THEME.fx;
    materials.skin.color.setHex(fx.skinTones[style.skin] ?? fx.skinTones[0]);
    materials.hair.color.setHex(fx.hairColors[style.hairColor] ?? fx.hairColors[0]);
    materials.hat.color.setHex(fx.hatColors[style.hatColor] ?? fx.hatColors[0]);
    materials.eyewear.color.setHex(fx.eyewearColors[style.eyewearColor] ?? fx.eyewearColors[0]);
    materials.wrist.color.setHex(fx.gearColors[style.wristColor] ?? fx.gearColors[0]);
    if (style.bootsColor >= 0) materials.boots.color.setHex(fx.bootColors[style.bootsColor] ?? fx.bootColors[0]); // < 0: keep the look's
    materials.stubble.color.setHex(fx.hairColors[style.hairColor] ?? fx.hairColors[0]);
  };
  let jersey: Jersey | null = null; // built on the first setJersey: NPCs never print one
  let ink = look.ink;
  let shirt = { number: 0, name: "" };
  const animator: Animator = createAnimator(rig);
  const triangles = countTriangles(rig.root);
  return {
    root: rig.root,
    rig,
    triangles,
    update: (pose, dt) => {
      animator.update(pose, dt);
      gaze.update(pose, dt);
      face.update(pose, dt, gaze.yaw, gaze.pitch);
      secondary.update(rig.root, pose.facing, dt);
      hands.update(pose, dt);
    },
    express: (emotion, holdS) => face.express(emotion, holdS),
    setLook: (l) => {
      applyLook(materials, l);
      ink = l.ink;
      jersey?.set(shirt.number, shirt.name, ink);
      styleColors(); // the team kit never overrides the player's own skin/hair
    },
    setStyle: (st) => {
      style = st;
      restyle(dressing, st, materials);
      restyleAccessories(extras, st, materials);
      if (jersey) jersey.mesh.scale.x = st.build; // the print widens with the torso
      styleColors();
    },
    setJersey: (number, name) => {
      shirt = { number, name };
      if (!jersey) {
        jersey = createJersey(rig.bones.chest, TORSO_DEPTH);
        jersey.mesh.scale.x = style?.build ?? 1;
      }
      jersey.set(number, name, ink);
    },
    dispose: () => {
      jersey?.dispose();
      rig.root.traverse((o) => {
        if (o instanceof THREE.Mesh) (o.geometry as THREE.BufferGeometry).dispose();
      });
      disposeMaterials(materials);
    },
  };
}

export type { CharacterPose, StrikePose } from "./animator";
export type { CharacterLook, StrikePart, StrikeTier } from "./config";
export type { Emotion } from "./expressions";
export type { CharacterStyle } from "./style";
export { DEFAULT_STYLE, styleFromSeed } from "./style";
export { BODY, DEFAULT_LOOK, PART_TIER, SIM_ZONES } from "./config";
