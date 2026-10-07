import * as THREE from "three";
import { THEME } from "../env/theme";
import { Snap } from "../snapshot";
import { padSign } from "./pad-sign";
import { LOBBY_HALF_X, LOBBY_HALF_Z, LOBBY_PAD_COUNT, LOBBY_PAD_RADIUS, PAD_LABELS, padX, padZ } from "./pads";

// Lobby decor (docs/design/lobby.md §7): floor, neon border, 3 pads with a
// billboard sign and a hold-progress ring. Built once; update() only writes
// uniforms, scales and opacities (allocation-free).
const FLOOR_Y = 0.001;
const PAD_Y = 0.01;
const RING_Y = 0.02;
const EDGE_W = 0.12;
const EDGE_H = 0.08;
const EDGE_BOOST = 2.2; // > 1 so the border crosses the bloom threshold
const GRID_STEP = 2;
const GRID_OPACITY = 0.12;
const DISC_OPACITY = 0.28;
const DISC_FLASH_OPACITY = 0.9;
const OUTLINE_W = 0.08;
const FILL_IN = 0.12;
const FILL_OUT = 0.38;
const SEGMENTS = 64;
const SIGN_Y = 3;
const PULSE_SCALE = 0.18;
const PULSE_DECAY = 4; // 1/s
const FLASH_DECAY = 2.5;
const IDLE_HZ = 0.6;
const IDLE_AMP = 0.06;
const PAD_COLORS: readonly number[] = [THEME.padTraining, THEME.pad1v1, THEME.pad2v2];

const FILL_VERT = "varying vec2 vP; void main() { vP = position.xy; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }";
const FILL_FRAG = `uniform float progress; uniform vec3 color; varying vec2 vP;
void main() {
  float a = atan(vP.x, vP.y) / 6.28318530718; // 0 at the top, clockwise
  if (a < 0.0) a += 1.0;
  if (a > progress) discard;
  gl_FragColor = vec4(color * 1.8, 1.0);
}`;

interface Pad {
  readonly disc: THREE.Mesh<THREE.CircleGeometry, THREE.MeshBasicMaterial>;
  readonly progress: { value: number };
  pulse: number;
  flash: number;
}

const flatMesh = <G extends THREE.BufferGeometry, M extends THREE.Material>(g: G, m: M, x: number, y: number, z: number): THREE.Mesh<G, M> => {
  const mesh = new THREE.Mesh(g, m);
  mesh.rotation.x = -Math.PI / 2;
  mesh.position.set(x, y, z);
  return mesh;
};

export class LobbyView {
  readonly group = new THREE.Group();
  private readonly pads: Pad[] = [];
  private time = 0;

  constructor(scene: THREE.Scene) {
    this.group.visible = false;
    this.buildFloor();
    for (let i = 0; i < LOBBY_PAD_COUNT; i++) this.pads.push(this.buildPad(i));
    scene.add(this.group);
  }

  set visible(v: boolean) { this.group.visible = v; }

  /** EvPadEnter: pulse; EvPadStart: flash (side = pad index). */
  pulse(pad: number): void { const p = this.pads[pad]; if (p) p.pulse = 1; }
  flash(pad: number): void { const p = this.pads[pad]; if (p) p.flash = 1; }

  update(curr: Float64Array, dt: number): void {
    this.time += dt;
    const active = curr[Snap.LobbyPad] | 0;
    const hold = Math.max(0, Math.min(1, curr[Snap.LobbyHold]));
    const idle = 1 + Math.sin(this.time * Math.PI * 2 * IDLE_HZ) * IDLE_AMP;
    for (let i = 0; i < this.pads.length; i++) {
      const p = this.pads[i];
      p.pulse = Math.max(0, p.pulse - dt * PULSE_DECAY);
      p.flash = Math.max(0, p.flash - dt * FLASH_DECAY);
      p.progress.value = i === active ? hold : 0;
      const s = idle + p.pulse * PULSE_SCALE;
      p.disc.scale.set(s, s, 1);
      p.disc.material.opacity = DISC_OPACITY + (DISC_FLASH_OPACITY - DISC_OPACITY) * p.flash;
    }
  }

  private buildFloor(): void {
    const w = LOBBY_HALF_X * 2;
    const d = LOBBY_HALF_Z * 2;
    const floor = flatMesh(new THREE.PlaneGeometry(w, d), new THREE.MeshStandardMaterial({ color: THEME.lobbyFloor, roughness: 0.85 }), 0, FLOOR_Y, 0);
    floor.receiveShadow = true;
    const grid = new THREE.GridHelper(Math.max(w, d), Math.max(w, d) / GRID_STEP, THEME.lobbyEdge, THEME.lobbyEdge);
    grid.scale.set(w / Math.max(w, d), 1, d / Math.max(w, d));
    grid.position.y = FLOOR_Y * 2;
    const gm = grid.material;
    if (gm instanceof THREE.Material) { gm.transparent = true; gm.opacity = GRID_OPACITY; }
    this.group.add(floor, grid);
    const edgeMat = new THREE.MeshBasicMaterial({ color: new THREE.Color(THEME.lobbyEdge).multiplyScalar(EDGE_BOOST) });
    const bar = (len: number, x: number, z: number, alongX: boolean): void => {
      const m = new THREE.Mesh(new THREE.BoxGeometry(alongX ? len : EDGE_W, EDGE_H, alongX ? EDGE_W : len), edgeMat);
      m.position.set(x, EDGE_H / 2, z);
      this.group.add(m);
    };
    bar(w, 0, LOBBY_HALF_Z, true);
    bar(w, 0, -LOBBY_HALF_Z, true);
    bar(d, LOBBY_HALF_X, 0, false);
    bar(d, -LOBBY_HALF_X, 0, false);
  }

  private buildPad(i: number): Pad {
    const x = padX(i);
    const z = padZ(i);
    const color = PAD_COLORS[i] ?? THEME.padFill;
    const disc = flatMesh(new THREE.CircleGeometry(LOBBY_PAD_RADIUS, SEGMENTS),
      new THREE.MeshBasicMaterial({ color, transparent: true, opacity: DISC_OPACITY, depthWrite: false }), x, PAD_Y, z);
    const outline = flatMesh(new THREE.RingGeometry(LOBBY_PAD_RADIUS - OUTLINE_W, LOBBY_PAD_RADIUS, SEGMENTS),
      new THREE.MeshBasicMaterial({ color: new THREE.Color(color).multiplyScalar(EDGE_BOOST) }), x, RING_Y, z);
    const progress = { value: 0 };
    const fillMat = new THREE.ShaderMaterial({
      uniforms: { progress, color: { value: new THREE.Color(THEME.padFill) } },
      vertexShader: FILL_VERT, fragmentShader: FILL_FRAG,
    });
    const fill = flatMesh(new THREE.RingGeometry(LOBBY_PAD_RADIUS + FILL_IN, LOBBY_PAD_RADIUS + FILL_OUT, SEGMENTS), fillMat, x, RING_Y, z);
    const sign = padSign(PAD_LABELS[i] ?? "", color);
    sign.position.set(x, SIGN_Y, z);
    this.group.add(disc, outline, fill, sign);
    return { disc, progress, pulse: 0, flash: 0 };
  }
}
