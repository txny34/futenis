import * as THREE from "three";
import { buildCourt } from "./court";
import { Arena } from "./env/arena";
import { LobbyView } from "./lobby/lobby-view";
import { Contact, Extra, PLAYER_CONTACT, PLAYER_EXTRA, PLAYER_POS, Phase, Snap, teamOf } from "./snapshot";
import type { Quality } from "./ui/settings";
import { BallView } from "./view/ball";
import { CameraRig } from "./view/camera";
import { ImpactFx } from "./view/impact-fx";
import { LandingMarker } from "./view/landing-marker";
import { MachineView } from "./view/machine";
import { TimingRing } from "./view/timing-ring";
import { Overlays } from "./view/overlays";
import { PostFx } from "./view/post";
import { PlayersView } from "./view/players";
import { addLights } from "./view/scene";
import { ServeBox } from "./view/serve-box";

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
  /** Landing marker allowed by settings ("Marcador de pique") for this match. */
  showLanding: boolean;
}

/** Normalized screen position (0..1, origin top-left); reused, never reallocated. */
export interface ScreenPoint { x: number; y: number }

export class CourtView {
  readonly players: PlayersView;
  readonly rig = new CameraRig();
  readonly fx: ImpactFx;
  readonly machine: MachineView;
  readonly lobby: LobbyView;
  private readonly court: THREE.Group;
  private inLobby = false;
  private readonly renderer: THREE.WebGLRenderer;
  private readonly scene = new THREE.Scene();
  private readonly sun: THREE.DirectionalLight;
  private readonly ball: BallView;
  private readonly overlays: Overlays;
  private readonly serveBox: ServeBox;
  private readonly humanFeet = new THREE.Vector3();
  private readonly arena: Arena;
  private readonly post: PostFx;
  private readonly timing: TimingRing;
  private readonly landing: LandingMarker;
  private readonly projected = new THREE.Vector3();

  constructor(private readonly container: HTMLElement) {
    this.renderer = new THREE.WebGLRenderer({ antialias: true, powerPreference: "high-performance" });
    this.renderer.shadowMap.enabled = true;
    this.renderer.shadowMap.type = THREE.PCFSoftShadowMap;
    container.appendChild(this.renderer.domElement);
    this.sun = addLights(this.scene);
    this.arena = new Arena(this.scene);
    this.court = buildCourt();
    this.scene.add(this.court);
    this.lobby = new LobbyView(this.scene);
    this.serveBox = new ServeBox(this.scene);
    this.ball = new BallView(this.scene);
    this.players = new PlayersView(this.scene);
    this.overlays = new Overlays(this.scene);
    this.timing = new TimingRing(this.scene);
    this.landing = new LandingMarker(this.scene);
    this.fx = new ImpactFx(this.scene);
    this.machine = new MachineView(this.scene);
    this.post = new PostFx(this.renderer, this.scene, this.rig.camera);
    window.addEventListener("resize", () => this.resize());
    this.applyVideo("high", true);
  }

  get canvas(): HTMLCanvasElement { return this.renderer.domElement; }
  get camera(): THREE.PerspectiveCamera { return this.rig.camera; }

  /** Not per frame: reconfigures pixel ratio, bloom and shadows. */
  applyVideo(quality: Quality, shadows: boolean): void {
    this.post.setQuality(quality);
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

  /** Lobby decor vs padel court (lobby.md §7). Not per frame: called on mode change. */
  setLobby(on: boolean): void {
    this.inLobby = on;
    this.court.visible = !on;
    this.lobby.visible = on;
    this.arena.setLobby(on);
    this.ball.setVisible(!on);
  }

  /** Draws interpolated state between prev and curr snapshots. Allocation-free. */
  render(prev: Float64Array, curr: Float64Array, alpha: number, dt: number, human: HumanView): void {
    if (this.inLobby) human.showOverlays = false; // no aim, timing ring or landing marker without a ball
    else this.ball.update(
      lerp(prev[Snap.BallX], curr[Snap.BallX], alpha),
      lerp(prev[Snap.BallY], curr[Snap.BallY], alpha),
      lerp(prev[Snap.BallZ], curr[Snap.BallZ], alpha),
      dt,
    );
    this.players.update(prev, curr, alpha, dt);
    const hasHuman = human.slot < (curr[Snap.NumPlayers] | 0);
    if (hasHuman) this.players.feet(human.slot, prev, curr, alpha, this.humanFeet);
    this.updateOverlays(curr, human, hasHuman);
    this.updatePrediction(prev, curr, alpha, human, hasHuman);
    this.fx.update(dt);
    this.machine.update(curr, dt);
    if (this.inLobby) this.lobby.update(curr, dt);
    const serving = human.showOverlays && hasHuman && curr[Snap.Phase] === Phase.Serve && curr[Snap.Server] === HUMAN_TEAM;
    this.serveBox.update(serving, curr[PLAYER_POS[human.slot] + 2], dt);
    this.rig.update(hasHuman ? this.humanFeet : null, this.ball.position, dt);
    this.arena.update(dt);
    this.post.render(dt);
  }

  /** Projects a world point to normalized screen coords into out (allocation-free). */
  screenOf(x: number, y: number, z: number, out: ScreenPoint): ScreenPoint {
    this.projected.set(x, y, z).project(this.rig.camera);
    out.x = (this.projected.x + 1) / 2;
    out.y = (1 - this.projected.y) / 2;
    return out;
  }

  /** Timing ring (PredictContact) + landing marker (PredictLanding), contact-v2 §22. */
  private updatePrediction(prev: Float64Array, curr: Float64Array, alpha: number, h: HumanView, hasHuman: boolean): void {
    const phase = curr[Snap.Phase];
    const live = h.showOverlays && hasHuman && (phase === Phase.Serve || phase === Phase.Rally);
    const c = PLAYER_CONTACT[h.slot];
    if (live && curr[c + Contact.Ok] === 1) {
      const ticks = prev[c + Contact.Ok] === 1 ? lerp(prev[c + Contact.Ticks], curr[c + Contact.Ticks], alpha) : curr[c + Contact.Ticks];
      this.timing.update(this.humanFeet, ticks, curr[c + Contact.Quality]);
    } else {
      this.timing.hide();
    }
    const side = teamOf(h.slot, curr[Snap.NumPlayers] | 0);
    const lx = curr[Snap.LandingX];
    const mine = side === 0 ? lx < 0 : lx > 0;
    if (live && h.showLanding && curr[Snap.LandingOk] === 1 && mine) {
      this.landing.update(lx, curr[Snap.LandingZ], curr[Snap.LandingTicks]);
    } else {
      this.landing.hide();
    }
  }

  private updateOverlays(curr: Float64Array, h: HumanView, hasHuman: boolean): void {
    const visible = h.showOverlays && hasHuman;
    this.overlays.setVisible(visible);
    if (!visible) return;
    const e = PLAYER_EXTRA[h.slot];
    this.overlays.update(this.humanFeet, h.targetX, h.targetZ, curr[e + Extra.Charge]);
  }

  private resize(): void {
    const w = this.container.clientWidth || window.innerWidth;
    const h = this.container.clientHeight || window.innerHeight;
    this.renderer.setSize(w, h);
    this.post.resize(w, h);
    this.rig.resize(w / h);
  }
}
