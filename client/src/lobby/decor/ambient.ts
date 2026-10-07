import * as THREE from "three";
import { THEME } from "../../env/theme";
import { rand } from "./util";

// Ambient life: drifting dust motes (one Points, shader-animated), sweeping
// fake-volumetric spotlight cones from the roof, and a purple/navy hemisphere
// fill that only exists while the lobby group is visible.
const DUST_COUNT = 700;
const DUST_BOX = { x: 36, y: 12, z: 28 } as const;
const CONE_H = 15;
const CONE_R = 3.2;
const CONE_Y = 14.5;
const CONES: ReadonlyArray<readonly [number, number, number]> = [[-10, -6, 0], [10, -6, 1.7], [-12, 6, 3.1], [12, 6, 4.6]];
const SWEEP_AMP = 0.22; // rad
const SWEEP_HZ = 0.025;
const HEMI_INTENSITY = 0.9;
const FILL_INTENSITY = 8;
const FILL_DIST = 30;

const DUST_VERT = `uniform float uTime; attribute vec3 aSeed; varying float vA;
void main() {
  vec3 box = vec3(${DUST_BOX.x.toFixed(1)}, ${DUST_BOX.y.toFixed(1)}, ${DUST_BOX.z.toFixed(1)});
  vec3 p = aSeed * box + vec3(sin(uTime * 0.2 + aSeed.y * 9.0), uTime * (0.08 + aSeed.x * 0.12), cos(uTime * 0.17 + aSeed.z * 7.0));
  p = mod(p, box) - vec3(box.x * 0.5, 0.0, box.z * 0.5);
  vA = 0.6 + 0.4 * sin(uTime * 0.25 * (0.5 + aSeed.z) + aSeed.x * 20.0); // < 0.1 Hz twinkle
  vec4 mv = modelViewMatrix * vec4(p, 1.0);
  gl_PointSize = clamp(22.0 / max(-mv.z, 1.0), 1.0, 32.0);
  if (mv.z > -0.3) gl_Position = vec4(2.0, 2.0, 2.0, 1.0); // behind/at camera: cull
  gl_Position = projectionMatrix * mv;
}`;
const DUST_FRAG = `uniform vec3 uColor; varying float vA;
void main() { float d = length(gl_PointCoord - 0.5); if (d > 0.5) discard; gl_FragColor = clamp(vec4(uColor, vA * (1.0 - d * 2.0) * 0.35), vec4(0.0), vec4(4.0)); }`;
const CONE_VERT = "varying float vY; void main() { vY = uv.y; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }";
const CONE_FRAG = `uniform vec3 uColor; varying float vY;
void main() { gl_FragColor = clamp(vec4(uColor, pow(vY, 1.6) * 0.035), vec4(0.0), vec4(4.0)); }`;

export class Ambient {
  readonly group = new THREE.Group();
  private readonly time = { value: 0 };
  private readonly cones: Array<{ readonly pivot: THREE.Object3D; readonly phase: number }> = [];

  constructor() {
    const D = THEME.lobbyDecor;
    this.group.add(new THREE.HemisphereLight(D.hemiSky, D.hemiGround, HEMI_INTENSITY));
    D.neon.forEach((c, i) => {
      const l = new THREE.PointLight(c, FILL_INTENSITY, FILL_DIST, 1.2);
      l.position.set((i - 1) * 11, 6, -11);
      this.group.add(l);
    });
    this.group.add(this.buildDust());
    const geo = new THREE.CylinderGeometry(0.15, CONE_R, CONE_H, 32, 1, true);
    geo.translate(0, -CONE_H / 2, 0); // pivot at the lamp
    const mat = new THREE.ShaderMaterial({
      uniforms: { uColor: { value: new THREE.Color(D.beam) } }, vertexShader: CONE_VERT, fragmentShader: CONE_FRAG,
      transparent: true, depthWrite: false, blending: THREE.AdditiveBlending, side: THREE.DoubleSide,
    });
    for (const [x, z, phase] of CONES) {
      const pivot = new THREE.Object3D();
      pivot.position.set(x, CONE_Y, z);
      pivot.add(new THREE.Mesh(geo, mat));
      this.group.add(pivot);
      this.cones.push({ pivot, phase });
    }
  }

  update(time: number): void {
    this.time.value = time;
    for (const c of this.cones) {
      const a = time * Math.PI * 2 * SWEEP_HZ + c.phase;
      c.pivot.rotation.set(Math.sin(a) * SWEEP_AMP, 0, Math.cos(a * 1.3) * SWEEP_AMP);
    }
  }

  private buildDust(): THREE.Points {
    const pos = new Float32Array(DUST_COUNT * 3);
    const seed = new Float32Array(DUST_COUNT * 3);
    for (let i = 0; i < DUST_COUNT * 3; i++) seed[i] = rand(i * 1.618);
    const geo = new THREE.BufferGeometry();
    geo.setAttribute("position", new THREE.BufferAttribute(pos, 3));
    geo.setAttribute("aSeed", new THREE.BufferAttribute(seed, 3));
    const pts = new THREE.Points(geo, new THREE.ShaderMaterial({
      uniforms: { uTime: this.time, uColor: { value: new THREE.Color(THEME.lobbyDecor.dust) } },
      vertexShader: DUST_VERT, fragmentShader: DUST_FRAG, transparent: true, depthWrite: false, blending: THREE.AdditiveBlending,
    }));
    pts.frustumCulled = false;
    return pts;
  }
}
