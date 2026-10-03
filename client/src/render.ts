import * as THREE from "three";
import { buildCourt } from "./court";
import { Extra, FLAG_STAMINA_LOCK, PLAYER_EXTRA, PLAYER_POS, Phase, Snap } from "./snapshot";
import type { Quality } from "./ui/settings";
import { BallView } from "./view/ball";
import { CameraRig } from "./view/camera";
import { Overlays } from "./view/overlays";
import { PlayersView } from "./view/players";
import { addLights } from "./view/scene";
import { ServeBox } from "./view/serve-box";

const BG_COLOR = 0x0b1220;
const MAX_PIXEL_RATIO = 2;
const PIXEL_RATIO: Readonly<Record<Quality, number>> = { low: 0.75, med: 1, high: MAX_PIXEL_RATIO };
const HUMAN_TEAM = 0;

const lerp = (a: number, b: number, t: number): number => a + (b - a) * t;

/** What the view needs to know about the human this frame. */
export interface HumanView {
  slot: number;
  targetX: number;
  targetZ: number;
  showOverlays: boolean;
}

export class CourtView {
  readonly players: PlayersView;
  readonly rig = new CameraRig();
  private readonly renderer: THREE.WebGLRenderer;
  private readonly scene = new THREE.Scene();
  private readonly sun: THREE.DirectionalLight;
  private readonly ball: BallView;
  private readonly overlays: Overlays;
  private readonly serveBox: ServeBox;
  private readonly humanFeet = new THREE.Vector3();

  constructor(private readonly container: HTMLElement) {
    this.renderer = new THREE.WebGLRenderer({ antialias: true });
    this.renderer.shadowMap.enabled = true;
    this.renderer.shadowMap.type = THREE.PCFSoftShadowMap;
    container.appendChild(this.renderer.domElement);
    this.scene.background = new THREE.Color(BG_COLOR);
    this.sun = addLights(this.scene);
    this.scene.add(buildCourt());
    this.serveBox = new ServeBox(this.scene);
    this.ball = new BallView(this.scene);
    this.players = new PlayersView(this.scene);
    this.overlays = new Overlays(this.scene);
    window.addEventListener("resize", () => this.resize());
    this.applyVideo("high", true);
  }

  get canvas(): HTMLCanvasElement { return this.renderer.domElement; }
  get camera(): THREE.PerspectiveCamera { return this.rig.camera; }

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

  /** Draws interpolated state between prev and curr snapshots. Allocation-free. */
  render(prev: Float64Array, curr: Float64Array, alpha: number, dt: number, human: HumanView): void {
    this.ball.update(
      lerp(prev[Snap.BallX], curr[Snap.BallX], alpha),
      lerp(prev[Snap.BallY], curr[Snap.BallY], alpha),
      lerp(prev[Snap.BallZ], curr[Snap.BallZ], alpha),
      dt,
    );
    this.players.update(prev, curr, alpha, dt);
    const hasHuman = human.slot < (curr[Snap.NumPlayers] | 0);
    if (hasHuman) this.players.feet(human.slot, prev, curr, alpha, this.humanFeet);
    this.updateOverlays(curr, human, hasHuman);
    const serving = human.showOverlays && hasHuman && curr[Snap.Phase] === Phase.Serve && curr[Snap.Server] === HUMAN_TEAM;
    this.serveBox.update(serving, curr[PLAYER_POS[human.slot] + 2], dt);
    this.rig.update(hasHuman ? this.humanFeet : null, this.ball.position, dt);
    this.renderer.render(this.scene, this.rig.camera);
  }

  private updateOverlays(curr: Float64Array, h: HumanView, hasHuman: boolean): void {
    const visible = h.showOverlays && hasHuman;
    this.overlays.setVisible(visible);
    if (!visible) return;
    const e = PLAYER_EXTRA[h.slot];
    const locked = ((curr[e + Extra.Flags] | 0) & FLAG_STAMINA_LOCK) !== 0;
    this.overlays.update(this.humanFeet, h.targetX, h.targetZ, curr[e + Extra.Stamina], locked, curr[e + Extra.Charge], this.rig.camera);
  }

  private resize(): void {
    const w = this.container.clientWidth || window.innerWidth;
    const h = this.container.clientHeight || window.innerHeight;
    this.renderer.setSize(w, h);
    this.rig.resize(w / h);
  }
}
