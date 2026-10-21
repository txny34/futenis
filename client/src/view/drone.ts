import * as THREE from "three";

// Drone mode (replays): a free camera. WASD flies on the view's plane, E/Space
// up, Q/Ctrl down, Shift faster, drag to look, wheel = speed. Inert while off.
const LOOK_RAD_PER_PX = 0.004;
const PITCH_LIMIT = 1.45;
const SPEED_DEFAULT = 6;
const SPEED_MIN = 1;
const SPEED_MAX = 30;
const WHEEL_STEP = 1.15;
const BOOST = 3;
const BOUNDS = { x: 30, yMin: 0.3, yMax: 25, z: 22 } as const;
const UI = "button, input, select, textarea, .fx-replay";

export class DroneCam {
  active = false;
  /** A menu is over the replay: input is ignored (and held keys dropped). */
  blocked = false;
  private yaw = 0;
  private pitch = 0;
  private speed = SPEED_DEFAULT;
  private dragging = false;
  private readonly keys = new Set<string>();
  private readonly pos = new THREE.Vector3();
  private readonly dir = new THREE.Vector3();
  private readonly look = new THREE.Vector3();

  constructor() {
    window.addEventListener("keydown", (e) => { if (this.active && !this.blocked && !inField(e.target)) this.keys.add(e.code); });
    window.addEventListener("keyup", (e) => { this.keys.delete(e.code); });
    window.addEventListener("blur", () => { this.keys.clear(); this.dragging = false; });
    window.addEventListener("pointerdown", (e) => { if (this.active && !this.blocked && !isUi(e.target)) this.dragging = true; });
    window.addEventListener("pointerup", () => { this.dragging = false; });
    window.addEventListener("pointermove", (e) => { if (this.active && this.dragging) this.turn(e.movementX, e.movementY); });
    window.addEventListener("wheel", (e) => {
      if (!this.active || this.blocked || isUi(e.target)) return;
      const k = e.deltaY < 0 ? WHEEL_STEP : 1 / WHEEL_STEP;
      this.speed = Math.max(SPEED_MIN, Math.min(SPEED_MAX, this.speed * k));
    }, { passive: true });
  }

  /** Takes over from cam's current pose. */
  start(cam: THREE.Camera): void {
    this.pos.copy(cam.position);
    cam.getWorldDirection(this.dir);
    this.yaw = Math.atan2(this.dir.z, this.dir.x);
    this.pitch = Math.asin(Math.max(-1, Math.min(1, this.dir.y)));
    this.active = true;
  }

  /** Drops held keys and any drag (a menu took the input). */
  stopMoving(): void {
    this.keys.clear();
    this.dragging = false;
  }

  stop(): void {
    this.active = false;
    this.dragging = false;
    this.keys.clear();
  }

  update(cam: THREE.Camera, dt: number): void {
    const k = this.keys;
    const fwd = (k.has("KeyW") ? 1 : 0) - (k.has("KeyS") ? 1 : 0);
    const side = (k.has("KeyD") ? 1 : 0) - (k.has("KeyA") ? 1 : 0);
    const up = (k.has("KeyE") || k.has("Space") ? 1 : 0) - (k.has("KeyQ") || k.has("ControlLeft") ? 1 : 0);
    const v = this.speed * (k.has("ShiftLeft") || k.has("ShiftRight") ? BOOST : 1) * dt;
    const c = Math.cos(this.yaw), s = Math.sin(this.yaw);
    this.pos.x = clamp(this.pos.x + (c * fwd - s * side) * v, -BOUNDS.x, BOUNDS.x);
    this.pos.z = clamp(this.pos.z + (s * fwd + c * side) * v, -BOUNDS.z, BOUNDS.z);
    this.pos.y = clamp(this.pos.y + up * v, BOUNDS.yMin, BOUNDS.yMax);
    const cp = Math.cos(this.pitch);
    this.look.set(this.pos.x + c * cp, this.pos.y + Math.sin(this.pitch), this.pos.z + s * cp);
    cam.position.copy(this.pos);
    cam.lookAt(this.look);
  }

  private turn(dx: number, dy: number): void {
    this.yaw += dx * LOOK_RAD_PER_PX;
    this.pitch = clamp(this.pitch - dy * LOOK_RAD_PER_PX, -PITCH_LIMIT, PITCH_LIMIT);
  }
}

const clamp = (v: number, lo: number, hi: number): number => Math.max(lo, Math.min(hi, v));
const isUi = (t: EventTarget | null): boolean => t instanceof Element && t.closest(UI) !== null;
const inField = (t: EventTarget | null): boolean => t instanceof HTMLInputElement || t instanceof HTMLTextAreaElement;
