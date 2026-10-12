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
const PUPIL_REST_X = 0;
const PUPIL_GAZE = 0.006; // m of pupil shift per rad of gaze
const EFFORT_CHARGE = 0.7;

export interface Face {
  /** Shows `emotion` for `holdS` seconds, then the pose decides again. */
  express(emotion: Emotion, holdS: number): void;
  /** gazeYaw/gazePitch (rad): where the head is looking, so the pupils lead it a little. */
  update(pose: CharacterPose, dt: number, gazeYaw?: number, gazePitch?: number): void;
}

interface Eye { readonly lid: THREE.Mesh; readonly pupil: THREE.Object3D; readonly brow: THREE.Mesh; readonly side: number }

/** Iris + pupil + catch-light, scaled together (pupil dilation reads as the whole eye's focus). */
function buildIris(m: LookMaterials, side: number): THREE.Group {
  const g = new THREE.Group();
  g.position.z = 0.024;
  g.add(new THREE.Mesh(new THREE.SphereGeometry(0.0165, 12, 8), m.iris));
  const pupil = new THREE.Mesh(new THREE.SphereGeometry(0.0095, 10, 6), m.pupil);
  pupil.position.z = 0.0105;
  g.add(pupil);
  const glint = new THREE.Mesh(new THREE.SphereGeometry(0.0042, 6, 4), m.eyeWhite);
  glint.position.set(side * 0.005 + 0.004, 0.006, 0.0165); // the light comes from the upper left
  g.add(glint);
  return g;
}

let seedCounter = 1;

function buildEye(head: THREE.Object3D, m: LookMaterials, side: number): Eye {
  const g = new THREE.Group();
  g.position.set(side * EYE_X, EYE_Y, FACE_Z - 0.01);
  g.scale.set(1, 1.2, 0.5);
  head.add(g);
  g.add(new THREE.Mesh(new THREE.SphereGeometry(EYE_R, 10, 8), m.eyeWhite));
  const pupil = buildIris(m, side);
  g.add(pupil);
  const lid = new THREE.Mesh(new THREE.SphereGeometry(EYE_R * 1.14, 12, 6, 0, Math.PI * 2, 0, Math.PI / 2), m.skin);
  g.add(lid);
  const brow = new THREE.Mesh(new THREE.BoxGeometry(0.052, 0.013, 0.016), m.hair);
  brow.position.set(side * EYE_X, BROW_Y, FACE_Z + BROW_FORWARD); // in front of the bangs
  head.add(brow);
  return { lid, pupil, brow, side };
}

function buildNose(head: THREE.Object3D, m: LookMaterials): void {
  const nose = new THREE.Mesh(new THREE.SphereGeometry(0.024, 10, 8), m.skin);
  nose.position.set(0, EYE_Y - 0.038, FACE_Z + 0.004);
  nose.scale.set(0.85, 0.75, 0.95);
  nose.castShadow = true;
  head.add(nose);
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
  buildNose(head, m);
  const tex = mouthTexture();
  m.mouth.map = tex;
  buildMouth(head, m.mouth);
  const cur = { lid: 0.82, browTilt: 0, browLift: 0, pupil: 1 };
  let held: Emotion | null = null;
  let holdLeft = 0;
  let shown: Emotion | null = null;
  let seed = Math.imul(seedCounter++, 0x9e3779b1) >>> 0;
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
    update(pose, dt, gazeYaw = 0, gazePitch = 0) {
      for (const eye of eyes) { // the pupils lead the head toward what it tracks
        eye.pupil.position.x = PUPIL_REST_X + gazeYaw * PUPIL_GAZE;
        eye.pupil.position.y = gazePitch * PUPIL_GAZE;
      }
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
