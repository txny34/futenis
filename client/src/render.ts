import * as THREE from "three";
import { buildCourt } from "./court";
import { Snap } from "./snapshot";

const BALL_R = 0.11;
const PLAYER_R = 0.35;
const PLAYER_H = 1.8;
const PLAYER_COLORS = [0xe5484d, 0x3e7bfa] as const;
const BALL_COLOR = 0xf5e663;
const BALL_EMISSIVE = 0x332e00;
const BG_COLOR = 0x0d1117;
const CHARGE_COLOR = 0xffd23f;
// Tennis-game view: camera behind player 0's baseline (X=-10), looking down +X.
// Tuned for 16:9 with FOV 50: near baseline sits ~20 deg below view center
// (half-FOV 25) and the top of the rival back glass (X=10, Y=3) ~15 deg above.
const CAMERA_FOV = 50;
const CAMERA_NEAR = 0.1;
const CAMERA_FAR = 200;
const CAMERA_POS = { x: -19, y: 11, z: 0 } as const;
const CAMERA_TARGET = { x: 0, y: 0, z: 0 } as const;
const MAX_PIXEL_RATIO = 2;
const SKY_COLOR = 0xdbe9ff;
const GROUND_COLOR = 0x223344;
const HEMI_INTENSITY = 0.8;
const SUN_COLOR = 0xffffff;
const SUN_INTENSITY = 1.6;
const SUN_POS = { x: -6, y: 14, z: 8 } as const;
const SHADOW_MAP_SIZE = 2048;
const SHADOW_HALF_X = 12;
const SHADOW_HALF_Z = 8;
const SHADOW_FLOOR_Y = 0.003;
const SHADOW_OPACITY = 0.45;
const SHADOW_RADIUS_FACTOR = 1.3;
const SHADOW_FADE_HEIGHT = 6; // meters at which the ball shadow reaches min scale
const SHADOW_MIN_SCALE = 0.4;
const CHARGE_BAR_W = 0.8;
const CHARGE_BAR_H = 0.08;
const CHARGE_BAR_OFFSET_Y = 0.3;
const CHARGE_MIN_SCALE = 0.01;
const OVERLAY_RENDER_ORDER = 10;
const SPHERE_SEGMENTS = 24;
const SPHERE_RINGS = 16;
const CAPSULE_CAP_SEGMENTS = 8;
const CAPSULE_RADIAL_SEGMENTS = 16;

const lerp = (a: number, b: number, t: number): number => a + (b - a) * t;

export class CourtView {
  private readonly renderer: THREE.WebGLRenderer;
  private readonly scene = new THREE.Scene();
  private readonly camera = new THREE.PerspectiveCamera(CAMERA_FOV, 1, CAMERA_NEAR, CAMERA_FAR);
  private readonly ball: THREE.Mesh;
  private readonly ballShadow: THREE.Mesh;
  private readonly players: [THREE.Mesh, THREE.Mesh];
  private readonly chargeBars: [THREE.Mesh, THREE.Mesh];

  constructor(private readonly container: HTMLElement) {
    this.renderer = new THREE.WebGLRenderer({ antialias: true });
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, MAX_PIXEL_RATIO));
    this.renderer.shadowMap.enabled = true;
    this.renderer.shadowMap.type = THREE.PCFSoftShadowMap;
    container.appendChild(this.renderer.domElement);

    this.scene.background = new THREE.Color(BG_COLOR);
    this.camera.position.set(CAMERA_POS.x, CAMERA_POS.y, CAMERA_POS.z);
    this.camera.lookAt(CAMERA_TARGET.x, CAMERA_TARGET.y, CAMERA_TARGET.z);
    this.addLights();
    this.scene.add(buildCourt());

    this.ball = this.makeBall();
    this.ballShadow = this.makeBallShadow();
    this.players = [this.makePlayer(0), this.makePlayer(1)];
    this.chargeBars = [this.makeChargeBar(), this.makeChargeBar()];

    window.addEventListener("resize", () => this.resize());
    this.resize();
  }

  /** Draws interpolated state between prev and curr snapshots. Allocation-free. */
  render(prev: Float64Array, curr: Float64Array, alpha: number, charge0: number, charge1: number): void {
    const bx = lerp(prev[Snap.BallX], curr[Snap.BallX], alpha);
    const by = lerp(prev[Snap.BallY], curr[Snap.BallY], alpha);
    const bz = lerp(prev[Snap.BallZ], curr[Snap.BallZ], alpha);
    this.ball.position.set(bx, by, bz);
    this.ballShadow.position.set(bx, SHADOW_FLOOR_Y, bz);
    const s = Math.max(SHADOW_MIN_SCALE, 1 - by / SHADOW_FADE_HEIGHT);
    this.ballShadow.scale.set(s, s, s);

    this.placePlayer(0, prev, curr, alpha, Snap.P0X, charge0);
    this.placePlayer(1, prev, curr, alpha, Snap.P1X, charge1);
    this.renderer.render(this.scene, this.camera);
  }

  /** base = index of the player's X in the snapshot; Y and Z follow it. */
  private placePlayer(i: 0 | 1, prev: Float64Array, curr: Float64Array, alpha: number, base: number, charge: number): void {
    const x = lerp(prev[base], curr[base], alpha);
    const y = lerp(prev[base + 1], curr[base + 1], alpha);
    const z = lerp(prev[base + 2], curr[base + 2], alpha);
    // Sim Y is the player's feet; the capsule mesh is centered.
    this.players[i].position.set(x, y + PLAYER_H / 2, z);
    const bar = this.chargeBars[i];
    bar.visible = charge > 0;
    bar.scale.x = Math.max(CHARGE_MIN_SCALE, charge);
    bar.position.set(x, y + PLAYER_H + CHARGE_BAR_OFFSET_Y, z);
  }

  private addLights(): void {
    this.scene.add(new THREE.HemisphereLight(SKY_COLOR, GROUND_COLOR, HEMI_INTENSITY));
    const sun = new THREE.DirectionalLight(SUN_COLOR, SUN_INTENSITY);
    sun.position.set(SUN_POS.x, SUN_POS.y, SUN_POS.z);
    sun.castShadow = true;
    sun.shadow.mapSize.set(SHADOW_MAP_SIZE, SHADOW_MAP_SIZE);
    const cam = sun.shadow.camera;
    cam.left = -SHADOW_HALF_X;
    cam.right = SHADOW_HALF_X;
    cam.top = SHADOW_HALF_Z;
    cam.bottom = -SHADOW_HALF_Z;
    this.scene.add(sun);
  }

  private makeBall(): THREE.Mesh {
    const m = new THREE.Mesh(
      new THREE.SphereGeometry(BALL_R, SPHERE_SEGMENTS, SPHERE_RINGS),
      new THREE.MeshStandardMaterial({ color: BALL_COLOR, emissive: BALL_EMISSIVE }),
    );
    m.castShadow = true;
    this.scene.add(m);
    return m;
  }

  private makeBallShadow(): THREE.Mesh {
    const m = new THREE.Mesh(
      new THREE.CircleGeometry(BALL_R * SHADOW_RADIUS_FACTOR, SPHERE_SEGMENTS),
      new THREE.MeshBasicMaterial({ color: 0x000000, transparent: true, opacity: SHADOW_OPACITY, depthWrite: false }),
    );
    m.rotation.x = -Math.PI / 2;
    this.scene.add(m);
    return m;
  }

  private makePlayer(side: 0 | 1): THREE.Mesh {
    const m = new THREE.Mesh(
      new THREE.CapsuleGeometry(PLAYER_R, PLAYER_H - 2 * PLAYER_R, CAPSULE_CAP_SEGMENTS, CAPSULE_RADIAL_SEGMENTS),
      new THREE.MeshStandardMaterial({ color: PLAYER_COLORS[side], roughness: 0.6 }),
    );
    m.castShadow = true;
    this.scene.add(m);
    return m;
  }

  private makeChargeBar(): THREE.Mesh {
    const geo = new THREE.PlaneGeometry(CHARGE_BAR_W, CHARGE_BAR_H);
    geo.translate(CHARGE_BAR_W / 2, 0, 0); // grow from the left edge
    const m = new THREE.Mesh(geo, new THREE.MeshBasicMaterial({ color: CHARGE_COLOR, depthTest: false }));
    m.renderOrder = OVERLAY_RENDER_ORDER;
    m.visible = false;
    this.scene.add(m);
    return m;
  }

  private resize(): void {
    const w = this.container.clientWidth || window.innerWidth;
    const h = this.container.clientHeight || window.innerHeight;
    this.renderer.setSize(w, h);
    this.camera.aspect = w / h;
    this.camera.updateProjectionMatrix();
  }
}
