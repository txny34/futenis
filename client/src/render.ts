import * as THREE from "three";
import { buildCourt } from "./court";
import { Extra, FLAG_STAMINA_LOCK, PLAYER_EXTRA, Snap } from "./snapshot";
import type { Quality } from "./ui/settings";
import { Overlays } from "./view/overlays";
import { PlayersView } from "./view/players";
import { addLights, makeBall, makeBallShadow } from "./view/scene";

const BG_COLOR = 0x0d1117;
// Tennis-game view: camera behind team 0's baseline (X=-10), looking down +X.
const CAMERA_FOV = 50;
const CAMERA_NEAR = 0.1;
const CAMERA_FAR = 200;
const CAMERA_POS = { x: -19, y: 11, z: 0 } as const;
const MAX_PIXEL_RATIO = 2;
const PIXEL_RATIO: Readonly<Record<Quality, number>> = { low: 0.75, med: 1, high: MAX_PIXEL_RATIO };
const SHADOW_FLOOR_Y = 0.003;
const SHADOW_FADE_HEIGHT = 6;
const SHADOW_MIN_SCALE = 0.4;
const SHAKE_DURATION_S = 0.25;
const SHAKE_AMPLITUDE = 0.12;
const SHAKE_FREQ_X = 53;
const SHAKE_FREQ_Y = 71;

const lerp = (a: number, b: number, t: number): number => a + (b - a) * t;

/** What the view needs to know about the human this frame. */
export interface HumanView {
  slot: number;
  targetX: number;
  targetZ: number;
  showOverlays: boolean;
}

export class CourtView {
  readonly camera = new THREE.PerspectiveCamera(CAMERA_FOV, 1, CAMERA_NEAR, CAMERA_FAR);
  readonly players: PlayersView;
  private readonly renderer: THREE.WebGLRenderer;
  private readonly scene = new THREE.Scene();
  private readonly sun: THREE.DirectionalLight;
  private readonly ball: THREE.Mesh;
  private readonly ballShadow: THREE.Mesh;
  private readonly overlays: Overlays;
  private readonly humanFeet = new THREE.Vector3();
  private shakeLeft = 0;
  private time = 0;

  constructor(private readonly container: HTMLElement) {
    this.renderer = new THREE.WebGLRenderer({ antialias: true });
    this.renderer.shadowMap.enabled = true;
    this.renderer.shadowMap.type = THREE.PCFSoftShadowMap;
    container.appendChild(this.renderer.domElement);
    this.scene.background = new THREE.Color(BG_COLOR);
    this.placeCamera(0, 0);
    this.sun = addLights(this.scene);
    this.scene.add(buildCourt());
    this.ball = makeBall(this.scene);
    this.ballShadow = makeBallShadow(this.scene);
    this.players = new PlayersView(this.scene);
    this.overlays = new Overlays(this.scene);
    window.addEventListener("resize", () => this.resize());
    this.applyVideo("high", true);
  }

  get canvas(): HTMLCanvasElement { return this.renderer.domElement; }

  /** Not per frame: reconfigures pixel ratio and shadows. */
  applyVideo(quality: Quality, shadows: boolean): void {
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, PIXEL_RATIO[quality]));
    if (this.renderer.shadowMap.enabled !== shadows) {
      this.renderer.shadowMap.enabled = shadows;
      this.sun.castShadow = shadows;
      this.scene.traverse((o) => {
        if (o instanceof THREE.Mesh && o.material instanceof THREE.Material) o.material.needsUpdate = true;
      });
    }
    this.resize();
  }

  shake(): void { this.shakeLeft = SHAKE_DURATION_S; }

  /** Draws interpolated state between prev and curr snapshots. Allocation-free. */
  render(prev: Float64Array, curr: Float64Array, alpha: number, dt: number, human: HumanView): void {
    this.time += dt;
    const bx = lerp(prev[Snap.BallX], curr[Snap.BallX], alpha);
    const by = lerp(prev[Snap.BallY], curr[Snap.BallY], alpha);
    const bz = lerp(prev[Snap.BallZ], curr[Snap.BallZ], alpha);
    this.ball.position.set(bx, by, bz);
    this.ballShadow.position.set(bx, SHADOW_FLOOR_Y, bz);
    const s = Math.max(SHADOW_MIN_SCALE, 1 - by / SHADOW_FADE_HEIGHT);
    this.ballShadow.scale.set(s, s, s);
    this.players.update(prev, curr, alpha, dt);
    this.updateOverlays(prev, curr, alpha, human);
    this.updateShake(dt);
    this.renderer.render(this.scene, this.camera);
  }

  private updateOverlays(prev: Float64Array, curr: Float64Array, alpha: number, h: HumanView): void {
    const visible = h.showOverlays && h.slot < (curr[Snap.NumPlayers] | 0);
    this.overlays.setVisible(visible);
    if (!visible) return;
    const e = PLAYER_EXTRA[h.slot];
    this.players.feet(h.slot, prev, curr, alpha, this.humanFeet);
    const locked = ((curr[e + Extra.Flags] | 0) & FLAG_STAMINA_LOCK) !== 0;
    this.overlays.update(this.humanFeet, h.targetX, h.targetZ, curr[e + Extra.Stamina], locked, curr[e + Extra.Charge], this.camera);
  }

  private updateShake(dt: number): void {
    if (this.shakeLeft <= 0) return;
    this.shakeLeft = Math.max(0, this.shakeLeft - dt);
    const k = SHAKE_AMPLITUDE * (this.shakeLeft / SHAKE_DURATION_S);
    this.placeCamera(Math.sin(this.time * SHAKE_FREQ_X) * k, Math.sin(this.time * SHAKE_FREQ_Y) * k);
  }

  private placeCamera(dz: number, dy: number): void {
    this.camera.position.set(CAMERA_POS.x, CAMERA_POS.y + dy, CAMERA_POS.z + dz);
    this.camera.lookAt(0, 0, 0);
  }

  private resize(): void {
    const w = this.container.clientWidth || window.innerWidth;
    const h = this.container.clientHeight || window.innerHeight;
    this.renderer.setSize(w, h);
    this.camera.aspect = w / h;
    this.camera.updateProjectionMatrix();
  }
}
