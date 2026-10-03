import * as THREE from "three";
import { THEME } from "./theme";

// Stepped tribunes on both long sides and behind the rival back glass, filled
// with an instanced low-poly crowd. Idle bob runs in the vertex shader from a
// single time uniform: zero CPU work and zero allocations per frame.
const ROWS = 7;
const ROW_DEPTH = 0.9;
const ROW_RISE = 0.55;
const SIDE_START = 8.6; // |z| of the first row (LED board sits in between)
const SIDE_HALF_LEN = 15;
const BACK_START = 13.2; // x of the first row behind the rival glass
const BACK_HALF_LEN = 9;
const SEAT_SPACING = 0.75;
const FILL_RATE = 0.72;
const BODY_H = 0.75;
const BODY_R_TOP = 0.16;
const BODY_R_BOT = 0.22;
const HEAD_R = 0.15;
const LOW_POLY = 5;
const BOB_AMP = 0.06;
const BOB_SPEED = 3.2;
const JITTER = 0.18;
const SEED = 0x5eed;
const SCARF_SHARE = 0.6; // fraction of fans wearing a team scarf
const SCARF_W = 0.42;
const SCARF_H = 0.12;

/** Deterministic tiny PRNG (mulberry32) so the crowd layout is stable between loads. */
function rng(seed: number): () => number {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

interface Seat { readonly x: number; readonly y: number; readonly z: number; readonly rotY: number }

/** A tribune: rows run along `axis`, rising away from the court in direction `out`. */
interface Tribune { readonly cx: number; readonly cz: number; readonly alongX: boolean; readonly out: number; readonly start: number; readonly halfLen: number }

const TRIBUNES: readonly Tribune[] = [
  { cx: 0, cz: 0, alongX: true, out: -1, start: SIDE_START, halfLen: SIDE_HALF_LEN },
  { cx: 0, cz: 0, alongX: true, out: 1, start: SIDE_START, halfLen: SIDE_HALF_LEN },
  { cx: 0, cz: 0, alongX: false, out: 1, start: BACK_START, halfLen: BACK_HALF_LEN },
];

function rowCenter(t: Tribune, r: number): THREE.Vector3 {
  const d = t.out * (t.start + r * ROW_DEPTH + ROW_DEPTH / 2);
  const y = (r + 1) * ROW_RISE;
  return t.alongX ? new THREE.Vector3(t.cx, y, d) : new THREE.Vector3(d, y, t.cz);
}

function steps(): THREE.InstancedMesh {
  const mat = new THREE.MeshLambertMaterial(); // white: instance colors carry the tone
  const inst = new THREE.InstancedMesh(new THREE.BoxGeometry(1, 1, 1), mat, TRIBUNES.length * ROWS);
  const m = new THREE.Matrix4();
  const col = new THREE.Color();
  let i = 0;
  for (const t of TRIBUNES) {
    for (let r = 0; r < ROWS; r++) {
      const c = rowCenter(t, r);
      const len = 2 * t.halfLen;
      m.makeScale(t.alongX ? len : ROW_DEPTH, c.y, t.alongX ? ROW_DEPTH : len).setPosition(c.x, c.y / 2, c.z);
      inst.setMatrixAt(i, m);
      inst.setColorAt(i++, col.setHex(r % 2 === 0 ? THEME.standConcrete : THEME.standEdge));
    }
  }
  return inst;
}

function seats(): Seat[] {
  const rand = rng(SEED);
  const out: Seat[] = [];
  for (const t of TRIBUNES) {
    const rotY = t.alongX ? (t.out > 0 ? Math.PI : 0) : -Math.PI / 2;
    for (let r = 0; r < ROWS; r++) {
      const c = rowCenter(t, r);
      for (let s = -t.halfLen + SEAT_SPACING / 2; s < t.halfLen; s += SEAT_SPACING) {
        if (rand() > FILL_RATE) continue;
        const j = (rand() - 0.5) * JITTER;
        out.push(t.alongX ? { x: s + j, y: c.y, z: c.z, rotY } : { x: c.x, y: c.y, z: s + j, rotY });
      }
    }
  }
  return out;
}

/** Adds the shared bob to an instanced material (phase from the instance position). */
function withBob(mat: THREE.Material, time: { value: number }): void {
  mat.onBeforeCompile = (shader) => {
    shader.uniforms.uTime = time;
    shader.vertexShader = "uniform float uTime;\n" + shader.vertexShader.replace(
      "#include <begin_vertex>",
      `#include <begin_vertex>
       float ph = instanceMatrix[3].x * 1.7 + instanceMatrix[3].z * 2.3;
       transformed.y += max(0.0, sin(uTime * ${BOB_SPEED.toFixed(2)} + ph)) * ${BOB_AMP.toFixed(3)};`,
    );
  };
}

function bobbing(geo: THREE.BufferGeometry, count: number, time: { value: number }): THREE.InstancedMesh {
  const mat = new THREE.MeshLambertMaterial();
  withBob(mat, time);
  return new THREE.InstancedMesh(geo, mat, count);
}

function crowdGeos(): readonly [THREE.BufferGeometry, THREE.BufferGeometry, THREE.BufferGeometry] {
  const body = new THREE.CylinderGeometry(BODY_R_TOP, BODY_R_BOT, BODY_H, LOW_POLY);
  body.translate(0, BODY_H / 2, 0);
  const head = new THREE.IcosahedronGeometry(HEAD_R, 0);
  head.translate(0, BODY_H + HEAD_R * 0.9, 0);
  const scarf = new THREE.BoxGeometry(SCARF_W, SCARF_H, SCARF_W);
  scarf.translate(0, BODY_H - SCARF_H / 2, 0);
  return [body, head, scarf];
}

function crowd(list: Seat[], time: { value: number }): THREE.InstancedMesh[] {
  const rand = rng(SEED ^ 0x9e37);
  const [bodyGeo, headGeo, scarfGeo] = crowdGeos();
  const bodies = bobbing(bodyGeo, list.length, time);
  const heads = bobbing(headGeo, list.length, time);
  const scarves = bobbing(scarfGeo, list.length, time);
  const m = new THREE.Matrix4();
  const col = new THREE.Color();
  let nScarf = 0;
  list.forEach((s, i) => {
    m.makeRotationY(s.rotY).setPosition(s.x, s.y, s.z);
    bodies.setMatrixAt(i, m);
    heads.setMatrixAt(i, m);
    bodies.setColorAt(i, col.setHex(pick(THEME.crowdBody, rand)));
    heads.setColorAt(i, col.setHex(pick(THEME.crowdSkin, rand)));
    if (rand() >= SCARF_SHARE) return;
    scarves.setMatrixAt(nScarf, m);
    scarves.setColorAt(nScarf++, col.setHex(THEME.team[rand() < 0.5 ? 0 : 1]));
  });
  scarves.count = nScarf;
  return [bodies, heads, scarves];
}

const pick = (list: readonly number[], rand: () => number): number => list[(rand() * list.length) | 0];

export interface Stands {
  readonly group: THREE.Group;
  /** Shared time uniform for the crowd bob; the arena advances it. */
  readonly time: { value: number };
}

export function buildStands(): Stands {
  const time = { value: 0 };
  const group = new THREE.Group();
  group.add(steps(), ...crowd(seats(), time));
  return { group, time };
}
