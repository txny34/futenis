import * as THREE from "three";
import { THEME } from "../../env/theme";
import { LOBBY_PAD_COUNT, LOBBY_PAD_RADIUS, padX, padZ } from "../pads";
import { padLabel } from "./pad-label";
import { NEON_BOOST, rand } from "./util";

// Pad dressing: a low plinth ring, an animated dashed rim (shader), a light
// beam rising from each pad and one Points system of motes rising inside the
// beams. All motion is in shaders: per frame only the shared time uniform and
// per-pad "heat" (pulse/flash) uniforms change.
const PLINTH_IN = LOBBY_PAD_RADIUS + 0.05;
const PLINTH_OUT = LOBBY_PAD_RADIUS + 0.55;
const PLINTH_H = 0.16;
const RIM_R = PLINTH_OUT - 0.04;
const RIM_W = 0.07;
const BEAM_H = 7;
const MOTES_PER_PAD = 90;
const MOTE_RISE = 0.6; // m/s
const SEG = 64;
const LABEL_Y = 4.0;
const LABEL_BOB = 0.15;
const LABEL_HZ = 0.4;
const LABEL_HEAT_SCALE = 0.08;
const LABELS: ReadonlyArray<readonly [string, string]> = [["ENTRENÁ", "LANZAPELOTAS"], ["1v1", "CONTRA EL BOT"], ["2v2", "CONTRA BOTS"]];

const RIM_VERT = "varying vec2 vP; void main() { vP = position.xy; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }";
const RIM_FRAG = `uniform float uTime; uniform float uHeat; uniform vec3 uColor; varying vec2 vP;
void main() {
  float dash = smoothstep(0.3, 0.7, fract(atan(vP.y, vP.x) / 6.28318 * 24.0 - uTime * 0.12));
  gl_FragColor = clamp(vec4(uColor * (0.55 + dash * 0.25 + uHeat * 0.4), 1.0), vec4(0.0), vec4(4.0));
}`;
const BEAM_FRAG = `uniform float uTime; uniform float uHeat; uniform vec3 uColor; varying vec2 vUv;
void main() {
  float fade = pow(1.0 - vUv.y, 2.2);
  float bands = 0.9 + 0.1 * sin(vUv.y * 12.0 - uTime * 0.6);
  gl_FragColor = clamp(vec4(uColor, fade * bands * (0.06 + uHeat * 0.06)), vec4(0.0), vec4(4.0));
}`;
const UV_VERT = "varying vec2 vUv; void main() { vUv = uv; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }";
const MOTE_VERT = `uniform float uTime; attribute vec3 aSeed; attribute vec3 aColor; varying vec3 vColor; varying float vA;
void main() {
  float h = mod(aSeed.z * ${BEAM_H.toFixed(1)} + uTime * ${MOTE_RISE.toFixed(2)} * (0.6 + aSeed.y), ${BEAM_H.toFixed(1)});
  float a = aSeed.x * 6.2831 + uTime * 0.5;
  float r = ${LOBBY_PAD_RADIUS.toFixed(2)} * (0.25 + 0.7 * aSeed.y);
  vec3 p = position + vec3(cos(a) * r, h, sin(a) * r);
  vColor = aColor; vA = 1.0 - h / ${BEAM_H.toFixed(1)};
  vec4 mv = modelViewMatrix * vec4(p, 1.0);
  gl_PointSize = clamp(70.0 / max(-mv.z, 1.0), 1.0, 32.0);
  if (mv.z > -0.3) gl_Position = vec4(2.0, 2.0, 2.0, 1.0); // behind/at camera: cull
  gl_Position = projectionMatrix * mv;
}`;
const MOTE_FRAG = `varying vec3 vColor; varying float vA;
void main() { float d = length(gl_PointCoord - 0.5); if (d > 0.5) discard; gl_FragColor = clamp(vec4(vColor * 0.8, vA * 0.5 * (1.0 - d * 2.0)), vec4(0.0), vec4(4.0)); }`;

export const PAD_COLORS: readonly number[] = [THEME.padTraining, THEME.pad1v1, THEME.pad2v2];

export class PadFx {
  readonly group = new THREE.Group();
  private readonly time = { value: 0 };
  private readonly heat: Array<{ value: number }> = [];
  private readonly labels: THREE.Sprite[] = [];
  private readonly labelBase: number[] = [];

  constructor() {
    const plinthGeo = new THREE.CylinderGeometry(PLINTH_OUT, PLINTH_OUT, PLINTH_H, SEG, 1, true);
    const plinthMat = new THREE.MeshStandardMaterial({ color: THEME.lobbyDecor.plinth, roughness: 0.35, metalness: 0.7, side: THREE.DoubleSide });
    const bandMat = new THREE.MeshBasicMaterial({ color: new THREE.Color(THEME.lobbyDecor.padRim).multiplyScalar(NEON_BOOST), side: THREE.DoubleSide });
    const topGeo = new THREE.RingGeometry(PLINTH_IN, PLINTH_OUT, SEG);
    const rimGeo = new THREE.RingGeometry(RIM_R - RIM_W, RIM_R, SEG, 1);
    const beamGeo = new THREE.CylinderGeometry(LOBBY_PAD_RADIUS, LOBBY_PAD_RADIUS * 1.15, BEAM_H, SEG, 1, true);
    for (let i = 0; i < LOBBY_PAD_COUNT; i++) {
      const x = padX(i);
      const z = padZ(i);
      const heat = { value: 0 };
      this.heat.push(heat);
      const color = { value: new THREE.Color(PAD_COLORS[i]) };
      const side = new THREE.Mesh(plinthGeo, bandMat);
      side.position.set(x, PLINTH_H / 2, z);
      const top = new THREE.Mesh(topGeo, plinthMat);
      top.rotation.x = -Math.PI / 2;
      top.position.set(x, PLINTH_H + 0.001, z);
      const rim = new THREE.Mesh(rimGeo, new THREE.ShaderMaterial({ uniforms: { uTime: this.time, uHeat: heat, uColor: color }, vertexShader: RIM_VERT, fragmentShader: RIM_FRAG }));
      rim.rotation.x = -Math.PI / 2;
      rim.position.set(x, PLINTH_H + 0.004, z);
      const beam = new THREE.Mesh(beamGeo, new THREE.ShaderMaterial({
        uniforms: { uTime: this.time, uHeat: heat, uColor: { value: new THREE.Color(PAD_COLORS[i]).multiplyScalar(NEON_BOOST) } },
        vertexShader: UV_VERT, fragmentShader: BEAM_FRAG,
        transparent: true, depthWrite: false, blending: THREE.AdditiveBlending, side: THREE.DoubleSide,
      }));
      beam.position.set(x, BEAM_H / 2, z);
      const [title, sub] = LABELS[i] ?? ["", ""];
      const label = padLabel(title, sub, PAD_COLORS[i]);
      label.position.set(x, LABEL_Y, z);
      this.labels.push(label);
      this.labelBase.push(label.scale.x);
      this.group.add(side, top, rim, beam, label);
    }
    this.group.add(this.buildMotes());
  }

  /** heat 0..1 per pad (pulse/flash/hold): brightens rim and beam. */
  setHeat(pad: number, h: number): void { const u = this.heat[pad]; if (u) u.value = h; }

  update(time: number): void {
    this.time.value = time;
    for (let i = 0; i < this.labels.length; i++) {
      const l = this.labels[i];
      l.position.y = LABEL_Y + Math.sin(time * Math.PI * 2 * LABEL_HZ + i * 2.1) * LABEL_BOB;
      const k = 1 + this.heat[i].value * LABEL_HEAT_SCALE;
      const w = this.labelBase[i] * k;
      l.scale.set(w, w * (l.scale.y / l.scale.x), 1);
    }
  }

  private buildMotes(): THREE.Points {
    const n = MOTES_PER_PAD * LOBBY_PAD_COUNT;
    const pos = new Float32Array(n * 3);
    const seed = new Float32Array(n * 3);
    const col = new Float32Array(n * 3);
    const c = new THREE.Color();
    for (let i = 0; i < n; i++) {
      const pad = Math.floor(i / MOTES_PER_PAD);
      pos.set([padX(pad), 0, padZ(pad)], i * 3);
      seed.set([rand(i), rand(i + 0.37), rand(i + 0.71)], i * 3);
      c.set(PAD_COLORS[pad]);
      col.set([c.r, c.g, c.b], i * 3);
    }
    const geo = new THREE.BufferGeometry();
    geo.setAttribute("position", new THREE.BufferAttribute(pos, 3));
    geo.setAttribute("aSeed", new THREE.BufferAttribute(seed, 3));
    geo.setAttribute("aColor", new THREE.BufferAttribute(col, 3));
    const pts = new THREE.Points(geo, new THREE.ShaderMaterial({
      uniforms: { uTime: this.time }, vertexShader: MOTE_VERT, fragmentShader: MOTE_FRAG,
      transparent: true, depthWrite: false, blending: THREE.AdditiveBlending,
    }));
    pts.frustumCulled = false;
    return pts;
  }
}
