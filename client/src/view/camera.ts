import * as THREE from "three";

// Roblox-style third-person orbit camera around the human. Yaw/pitch come
// straight from mouse deltas (pointer lock) and are applied immediately; only
// the follow target and the zoom distance are damped. yaw = 0 looks down +X
// (toward the rival), pitch > 0 looks down at the court. Falls back to a fixed
// broadcast view when there is no human or the dynamic camera is off.
const FOV = 70;
const NEAR = 0.1;
const FAR = 200;
const FIXED_POS = { x: -15.5, y: 6.2, z: 0 } as const;
const FIXED_LOOK = { x: 1.5, y: 0.6, z: 0 } as const;
const HEAD_Y = 1.9; // look target just above the head: low over-the-shoulder view (reference video)
const RAD_PER_PX = 0.0025; // at sensitivity 1
const PITCH_MIN = -0.3;
const PITCH_MAX = 1.35;
const DEFAULT_PITCH = 0.34; // looks down at the court; the raised crosshair still lands deep in the rival half
const DIST_MIN = 2;
const DIST_MAX = 14;
const DEFAULT_DIST = 3.6; // close behind the player: they fill the lower third, the court feels big
const ZOOM_PER_DELTA = 0.006; // wheel deltaY -> meters
const MIN_CAM_Y = 0.25; // never under the floor
const FOLLOW_RATE = 14; // 1/s target damping
const ZOOM_RATE = 10;
const SHAKE_DURATION_S = 0.25;
const SHAKE_AMPLITUDE = 0.1;
const SHAKE_FREQ_A = 53;
const SHAKE_FREQ_B = 71;

const damp = (rate: number, dt: number): number => 1 - Math.exp(-rate * dt);
const clamp = (v: number, lo: number, hi: number): number => Math.max(lo, Math.min(hi, v));

export class CameraRig {
  readonly camera = new THREE.PerspectiveCamera(FOV, 1, NEAR, FAR);
  dynamic = true;
  sensitivity = 1;
  private yaw = 0;
  private pitch = DEFAULT_PITCH;
  private dist = DEFAULT_DIST;
  private goalDist = DEFAULT_DIST;
  private hasTarget = false;
  private readonly target = new THREE.Vector3();
  private shakeLeft = 0;
  private time = 0;

  constructor() { this.applyFixed(); }

  shake(): void { this.shakeLeft = SHAKE_DURATION_S; }

  /** Mouse movement in pixels (pointer lock movementX/Y). */
  look(dx: number, dy: number): void {
    const k = RAD_PER_PX * this.sensitivity;
    this.yaw += dx * k;
    this.pitch = clamp(this.pitch + dy * k, PITCH_MIN, PITCH_MAX);
  }

  /** Snaps the orbit to look along yaw (0 = +X, -PI/2 = -Z) with the default pitch. */
  face(yaw: number): void {
    this.yaw = yaw;
    this.pitch = DEFAULT_PITCH;
    this.hasTarget = false;
  }

  /** Wheel deltaY: positive = zoom out. */
  zoom(deltaY: number): void {
    this.goalDist = clamp(this.goalDist + deltaY * ZOOM_PER_DELTA, DIST_MIN, DIST_MAX);
  }

  /** human = interpolated feet (null if no human on court); ball = ball center (unused, kept for API). */
  update(human: THREE.Vector3 | null, _ball: THREE.Vector3, dt: number): void {
    this.time += dt;
    if (this.dynamic && human) this.orbit(human, dt);
    else {
      this.hasTarget = false;
      this.applyFixed();
    }
    this.applyShake(dt);
  }

  resize(aspect: number): void {
    this.camera.aspect = aspect;
    this.camera.updateProjectionMatrix();
  }

  private orbit(human: THREE.Vector3, dt: number): void {
    if (this.hasTarget) {
      const k = damp(FOLLOW_RATE, dt);
      this.target.x += (human.x - this.target.x) * k;
      this.target.y += (human.y + HEAD_Y - this.target.y) * k;
      this.target.z += (human.z - this.target.z) * k;
    } else {
      this.target.set(human.x, human.y + HEAD_Y, human.z);
      this.hasTarget = true;
    }
    this.dist += (this.goalDist - this.dist) * damp(ZOOM_RATE, dt);
    const cp = Math.cos(this.pitch);
    const p = this.camera.position;
    p.set(
      this.target.x - Math.cos(this.yaw) * cp * this.dist,
      this.target.y + Math.sin(this.pitch) * this.dist,
      this.target.z - Math.sin(this.yaw) * cp * this.dist,
    );
    if (p.y < MIN_CAM_Y) p.y = MIN_CAM_Y;
    this.camera.lookAt(this.target);
  }

  private applyFixed(): void {
    this.camera.position.set(FIXED_POS.x, FIXED_POS.y, FIXED_POS.z);
    this.camera.lookAt(FIXED_LOOK.x, FIXED_LOOK.y, FIXED_LOOK.z);
  }

  private applyShake(dt: number): void {
    if (this.shakeLeft <= 0) return;
    this.shakeLeft = Math.max(0, this.shakeLeft - dt);
    const k = SHAKE_AMPLITUDE * (this.shakeLeft / SHAKE_DURATION_S);
    this.camera.position.y += Math.sin(this.time * SHAKE_FREQ_A) * k;
    this.camera.position.z += Math.sin(this.time * SHAKE_FREQ_B) * k;
  }
}
