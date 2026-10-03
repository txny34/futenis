import * as THREE from "three";
import { Animator, CharacterPose, createAnimator } from "./animator";
import { CharacterLook, DEFAULT_LOOK } from "./config";
import { LookMaterials, applyLook, createMaterials, disposeMaterials } from "./materials";
import { countTriangles, dressRig } from "./parts";
import { Rig, createRig } from "./rig";

// Public entry point: a procedural toy-athlete puppet. The caller positions
// `root` (feet at local Y=0) from the interpolated snapshot and calls
// update() once per render frame.

export interface Character {
  readonly root: THREE.Object3D;
  readonly rig: Rig;
  readonly triangles: number;
  update(pose: CharacterPose, dt: number): void;
  setLook(look: CharacterLook): void;
  dispose(): void;
}

export function createCharacter(look: CharacterLook = DEFAULT_LOOK): Character {
  const rig = createRig();
  const materials: LookMaterials = createMaterials(look);
  dressRig(rig, materials);
  const animator: Animator = createAnimator(rig);
  const triangles = countTriangles(rig.root);
  return {
    root: rig.root,
    rig,
    triangles,
    update: (pose, dt) => animator.update(pose, dt),
    setLook: (l) => applyLook(materials, l),
    dispose: () => {
      rig.root.traverse((o) => {
        if (o instanceof THREE.Mesh) (o.geometry as THREE.BufferGeometry).dispose();
      });
      disposeMaterials(materials);
    },
  };
}

export type { CharacterPose, StrikePose } from "./animator";
export type { CharacterLook, StrikePart, StrikeTier } from "./config";
export { BODY, DEFAULT_LOOK, PART_TIER, SIM_ZONES } from "./config";
