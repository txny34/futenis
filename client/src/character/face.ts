import * as THREE from "three";
import type { CharacterPose } from "./animator";
import { BODY } from "./config";
import { EXPRESSIONS, Emotion } from "./expressions";
import { BROW_Y, EYE_R, EYE_X, EYE_Y, FACE_Z, HEAD_SCALE_XZ, HEAD_SCALE_Y, HEAD_Y, MOUTH_Y } from "./head";
import { LookMaterials } from "./materials";
import { mouthTexture, showMouth } from "./mouth-atlas";

// Expressive face (Schedule I style): 3D eyes with skin-colored upper lids,
// little brows, and a mouth decal from a texture atlas. Emotions come from
// game events (express) or from the pose (charging = focus, swinging =
// effort); the face blends lids/brows/pupils toward the preset and blinks on
// its own. Zero allocations per update.

const LID_OPEN_ROT = -0.5; // upper lid rolled back: eye wide open
const LID_SHUT_ROT = 1.62; // rolled over the front: eye closed
const BROW_REST_TILT = 0.12;
const BROW_FORWARD = 0.012;
const BLEND_RATE = 14; // 1/s
const BLINK_S = 0.12;
const BLINK_MIN_S = 2.2;
const BLINK_SPAN_S = 3.2;
const MOUTH_W = 0.56; // radians of the head sphere the mouth patch spans
const MOUTH_H = 0.3;
const FOCUS_CHARGE = 0.15;
const EFFORT_CHARGE = 0.7;

export interface Face {
  /** Shows `emotion` for `holdS` seconds, then the pose decides again. */
  express(emotion: Emotion, holdS: number): void;
  update(pose: CharacterPose, dt: number): void;
}

interface Eye { readonly lid: THREE.Mesh; readonly pupil: THREE.Mesh; readonly brow: THREE.Mesh; readonly side: number }

let seedCounter = 1;

function buildEye(head: THREE.Object3D, m: LookMaterials, side: number): Eye {
  const g = new THREE.Group();
  g.position.set(side * EYE_X, EYE_Y, FACE_Z - 0.01);
  g.scale.set(1, 1.2, 0.5);
  head.add(g);
  g.add(new THREE.Mesh(new THREE.SphereGeometry(EYE_R, 10, 8), m.eyeWhite));
  const pupil = new THREE.Mesh(new THREE.SphereGeometry(0.017, 8, 6), m.pupil);
  pupil.position.z = 0.028;
  pupil.scale.set(1, 1.08, 1);
  g.add(pupil);
  const lid = new THREE.Mesh(new THREE.SphereGeometry(EYE_R * 1.14, 12, 6, 0, Math.PI * 2, 0, Math.PI / 2), m.skin);
  g.add(lid);
  const brow = new THREE.Mesh(new THREE.BoxGeometry(0.052, 0.013, 0.016), m.hair);
  brow.position.set(side * EYE_X, BROW_Y, FACE_Z + BROW_FORWARD); // in front of the bangs
  head.add(brow);
  return { lid, pupil, brow, side };
}

function buildMouth(head: THREE.Object3D, mat: THREE.Material): void {
  const r = BODY.headRadius * 1.012;
  const theta = Math.acos((MOUTH_Y - HEAD_Y) / r);
  const geo = new THREE.SphereGeometry(r, 10, 4, Math.PI / 2 - MOUTH_W / 2, MOUTH_W, theta - MOUTH_H / 2, MOUTH_H);
  const mouth = new THREE.Mesh(geo, mat);
  mouth.position.y = HEAD_Y;
  mouth.scale.set(HEAD_SCALE_XZ, HEAD_SCALE_Y, HEAD_SCALE_XZ);
  head.add(mouth);
}

export function createFace(head: THREE.Object3D, m: LookMaterials): Face {
  const eyes = [buildEye(head, m, 1), buildEye(head, m, -1)];
  const tex = mouthTexture();
  m.mouth.map = tex;
  buildMouth(head, m.mouth);
  const cur = { lid: 0.82, browTilt: 0, browLift: 0, pupil: 1 };
  let held: Emotion | null = null;
  let holdLeft = 0;
  let shown: Emotion | null = null;
  let seed = (seedCounter++ * 2654435761) >>> 0;
  const rand = (): number => {
    seed = (seed * 1664525 + 1013904223) >>> 0;
    return seed / 4294967296;
  };
  let blinkIn = BLINK_MIN_S + rand() * BLINK_SPAN_S;

  const pick = (pose: CharacterPose): Emotion => {
    if (held !== null) return held;
    if (pose.strike !== null && pose.strike.t > 0) return "effort";
    if (pose.charge >= EFFORT_CHARGE) return "effort";
    return pose.charge >= FOCUS_CHARGE ? "focus" : "neutral";
  };

  const apply = (blink: boolean): void => {
    const lid = blink ? 0 : cur.lid;
    for (const eye of eyes) {
      eye.lid.rotation.x = LID_SHUT_ROT + (LID_OPEN_ROT - LID_SHUT_ROT) * lid;
      eye.pupil.scale.set(cur.pupil, cur.pupil * 1.08, 1);
      eye.brow.rotation.z = -eye.side * (BROW_REST_TILT + cur.browTilt);
      eye.brow.position.y = BROW_Y + cur.browLift;
    }
  };

  return {
    express(emotion, holdS) {
      held = emotion;
      holdLeft = holdS;
    },
    update(pose, dt) {
      if (held !== null && (holdLeft -= dt) <= 0) held = null;
      const emotion = pick(pose);
      const e = EXPRESSIONS[emotion];
      if (emotion !== shown) {
        shown = emotion;
        showMouth(tex, e.mouth);
      }
      const k = 1 - Math.exp(-dt * BLEND_RATE);
      cur.lid += (e.lid - cur.lid) * k;
      cur.browTilt += (e.browTilt - cur.browTilt) * k;
      cur.browLift += (e.browLift - cur.browLift) * k;
      cur.pupil += (e.pupil - cur.pupil) * k;
      blinkIn -= dt;
      if (blinkIn < -BLINK_S) blinkIn = BLINK_MIN_S + rand() * BLINK_SPAN_S;
      apply(blinkIn < 0);
    },
  };
}
