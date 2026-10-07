import * as THREE from "three";
import { THEME } from "../env/theme";
import { Snap } from "../snapshot";
import { LobbyDecor } from "./decor/decor";
import { PAD_COLORS } from "./decor/pad-fx";
import { padSign } from "./pad-sign";
import { LOBBY_HALF_X, LOBBY_HALF_Z, LOBBY_PAD_COUNT, LOBBY_PAD_RADIUS, PAD_LABELS, padX, padZ } from "./pads";

// Lobby view (docs/design/lobby.md §7): neon border, 3 pads with a bobbing
// billboard sign and a hold-progress ring, plus the decor pass (decor/*).
// Built once; update() only writes uniforms, scales and positions (allocation-free).
const PAD_Y = 0.01;
const RING_Y = 0.02;
const FILL_Y = 0.17; // above the decor plinth (decor/pad-fx.ts)
const EDGE_W = 0.12;
const EDGE_H = 0.08;
const EDGE_BOOST = 0.7; // > 1 so the border crosses the bloom threshold
const DISC_OPACITY = 0.28;
const DISC_FLASH_OPACITY = 0.36; // no start flash: barely brighter
const OUTLINE_W = 0.08;
const FILL_IN = 0.12;
const FILL_OUT = 0.38;
const SEGMENTS = 64;
const SIGN_Y = 3.4;
const SIGN_BOB = 0.15;
const SIGN_BOB_HZ = 0.5;
const PULSE_SCALE = 0.05;
const PULSE_DECAY = 4; // 1/s
const FLASH_DECAY = 2.5;
const IDLE_HZ = 0.2;
const IDLE_AMP = 0.015;

const FILL_VERT = "varying vec2 vP; void main() { vP = position.xy; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }";
const FILL_FRAG = `uniform float progress; uniform vec3 color; varying vec2 vP;
void main() {
  float a = atan(vP.x, vP.y) / 6.28318530718; // 0 at the top, clockwise
  if (a < 0.0) a += 1.0;
  if (a > progress) discard;
  gl_FragColor = vec4(color * 1.1, 1.0);
}`;

interface Pad {
  readonly disc: THREE.Mesh<THREE.CircleGeometry, THREE.MeshBasicMaterial>;
  readonly progress: { value: number };
  readonly sign: THREE.Sprite;
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
  private readonly decor = new LobbyDecor(LOBBY_HALF_X, LOBBY_HALF_Z);
  private time = 0;

  constructor(scene: THREE.Scene) {
    this.group.visible = false;
    this.group.add(this.decor.group);
    this.buildEdge();
    for (let i = 0; i < LOBBY_PAD_COUNT; i++) this.pads.push(this.buildPad(i));
    scene.add(this.group);
  }

  set visible(v: boolean) { this.group.visible = v; }

  /** EvPadEnter: pulse; EvPadStart: flash (side = pad index). */
  pulse(pad: number): void { const p = this.pads[pad]; if (p) p.pulse = 1; }
  flash(pad: number): void { const p = this.pads[pad]; if (p) { p.flash = 1; this.decor.burst(pad); } }

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
      p.sign.position.y = SIGN_Y + Math.sin(this.time * Math.PI * 2 * SIGN_BOB_HZ + i * 2) * SIGN_BOB;
      this.decor.setPadHeat(i, p.progress.value);
    }
    this.decor.update(dt);
  }

  private buildEdge(): void {
    const w = LOBBY_HALF_X * 2;
    const d = LOBBY_HALF_Z * 2;
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
    const fill = flatMesh(new THREE.RingGeometry(LOBBY_PAD_RADIUS + FILL_IN, LOBBY_PAD_RADIUS + FILL_OUT, SEGMENTS), fillMat, x, FILL_Y, z);
    const sign = padSign(PAD_LABELS[i] ?? "", color);
    sign.position.set(x, SIGN_Y, z);
    sign.visible = false; // the big decor label (decor/pad-label.ts) is the pad marker now
    this.group.add(disc, outline, fill, sign);
    return { disc, progress, sign, pulse: 0, flash: 0 };
  }
}
