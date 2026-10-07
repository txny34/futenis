import * as THREE from "three";
import { THEME } from "../../env/theme";
import { rand } from "./util";

// Confetti burst (EvPadStart / pad enter): one preallocated Points system; a
// burst only rewrites two uniforms (origin, start time). Ballistic motion and
// fade are computed in the vertex shader.
const COUNT = 90;
const LIFE = 2.6; // s
const GRAVITY = 3.5;

const VERT = `uniform float uAge; uniform vec3 uOrigin; uniform float uPower; attribute vec3 aVel; attribute vec3 aColor;
varying vec3 vColor; varying float vA;
void main() {
  float t = uAge;
  vec3 v = aVel * uPower;
  vec3 p = uOrigin + v * t + vec3(sin(t * 6.0 + aVel.x * 10.0) * 0.2, -0.5 * ${GRAVITY.toFixed(1)} * t * t, 0.0);
  p.y = max(p.y, 0.03);
  vColor = aColor; vA = clamp(1.0 - t / ${LIFE.toFixed(1)}, 0.0, 1.0);
  vec4 mv = modelViewMatrix * vec4(p, 1.0);
  gl_PointSize = clamp(90.0 / max(-mv.z, 1.0), 1.0, 32.0);
  if (mv.z > -0.3) gl_Position = vec4(2.0, 2.0, 2.0, 1.0); // behind/at camera: cull
  gl_Position = projectionMatrix * mv;
}`;
const FRAG = `varying vec3 vColor; varying float vA;
void main() { vec2 q = abs(gl_PointCoord - 0.5); if (q.x > 0.5 || q.y > 0.22) discard; gl_FragColor = clamp(vec4(vColor * 0.7, vA * 0.8), vec4(0.0), vec4(4.0)); }`;

export class Confetti {
  readonly points: THREE.Points;
  private readonly age = { value: LIFE };
  private readonly origin = { value: new THREE.Vector3() };
  private readonly power = { value: 1 };

  constructor() {
    const pos = new Float32Array(COUNT * 3);
    const vel = new Float32Array(COUNT * 3);
    const col = new Float32Array(COUNT * 3);
    const c = new THREE.Color();
    const palette = THEME.lobbyDecor.confetti;
    for (let i = 0; i < COUNT; i++) {
      const a = rand(i) * Math.PI * 2;
      const s = 1 + rand(i + 0.5) * 2.5;
      vel.set([Math.cos(a) * s, 4 + rand(i + 0.9) * 5, Math.sin(a) * s], i * 3);
      c.set(palette[i % palette.length]);
      col.set([c.r, c.g, c.b], i * 3);
    }
    const geo = new THREE.BufferGeometry();
    geo.setAttribute("position", new THREE.BufferAttribute(pos, 3));
    geo.setAttribute("aVel", new THREE.BufferAttribute(vel, 3));
    geo.setAttribute("aColor", new THREE.BufferAttribute(col, 3));
    this.points = new THREE.Points(geo, new THREE.ShaderMaterial({
      uniforms: { uAge: this.age, uOrigin: this.origin, uPower: this.power }, vertexShader: VERT, fragmentShader: FRAG,
      transparent: true, depthWrite: false,
    }));
    this.points.frustumCulled = false;
    this.points.visible = false;
  }

  burst(x: number, z: number, power: number): void {
    this.origin.value.set(x, 0.2, z);
    this.power.value = power;
    this.age.value = 0;
    this.points.visible = true;
  }

  update(dt: number): void {
    if (!this.points.visible) return;
    this.age.value += dt;
    if (this.age.value >= LIFE) this.points.visible = false;
  }
}
