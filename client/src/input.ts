import * as THREE from "three";
import type { HumanInput } from "./engine";
import { clampUnit, GamepadReader, PAD_LOOK_PX_PER_S } from "./gamepad";
import { Btn } from "./snapshot";
import { DEFAULT_KEYS, type KeyBinds } from "./ui/keybinds";

// Single human on slot 0, Roblox-style: the mouse (pointer lock) turns the camera, WASD/left
// stick moves relative to the camera yaw, and the aim is wherever the crosshair (above the
// screen center, AIM_NDC_Y) hits the floor. Everything is a held LEVEL (PlayerInput v2); the
// sim detects edges itself. Spin is always 0. Keys are remappable (ui/keybinds.ts; WASD by default).
const MOUSE_LEFT = 1; // PointerEvent.buttons bit
const MOUSE_RIGHT = 2;
const AIM_NDC_Y = 0.25; // crosshair height in NDC = 37.5% from the top (hud.css #crosshair)
const DEFAULT_TARGET_X = 5; // middle of the rival half
const RIVAL_MIN_X = 0.5; // team 0 aims at X > 0
const COURT_HALF_Z = 5;
const MIN_FORWARD_X = 0.05; // camera facing away from the rival: aim straight ahead
const SHORT_TARGET_X = 1.5; // crosshair on our own half (looking down): a short drop past the net
const DEEP_TARGET_X = 9.5; // crosshair above the horizon: deepest shot
const LOW_AIM_X = -2.5; // crosshair this deep into our own half (aiming really low): the shot goes into the net

/** Keyboard + mouse -> HumanInput. Knows nothing about rules. Allocation-free per frame. */
export class Controls {
  /** Mouse delta while locked (pixels). */
  onLook: ((dx: number, dy: number) => void) | null = null;
  /** Wheel deltaY (positive = zoom out). */
  onZoom: ((deltaY: number) => void) | null = null;
  /** Pointer lock lost without the player asking (native Esc): the app pauses. */
  onLockLost: (() => void) | null = null;
  onLockChange: ((locked: boolean) => void) | null = null;
  onMenu: (() => void) | null = null; // pad Start button (rising edge): opens the pause/lobby menu
  private on = false;
  private locked = false;
  private freeRequested = false;
  private canvas: HTMLElement | null = null;
  private readonly down = new Set<string>();
  private mouseButtons = 0;
  private latched = 0; // buttons pressed since the last sample: a click shorter than a frame still counts
  private readonly center = new THREE.Vector2(0, AIM_NDC_Y);
  private readonly raycaster = new THREE.Raycaster();
  private readonly floor = new THREE.Plane(new THREE.Vector3(0, 1, 0), 0);
  private readonly hit = new THREE.Vector3();
  private readonly fwd = new THREE.Vector3();
  private readonly action: HumanInput = { moveX: 0, moveZ: 0, targetX: DEFAULT_TARGET_X, targetZ: 0, buttons: 0, spin: 0 };
  private pressHandler: ((code: string) => void) | null = null;
  private readonly pad = new GamepadReader();
  private keys: KeyBinds = DEFAULT_KEYS;
  private bound: ReadonlySet<string> = new Set(Object.values(DEFAULT_KEYS));

  /** Ajustes > Controles (ui/keybinds.ts). Held keys are dropped so nothing sticks. */
  setKeys(k: KeyBinds): void { if (k !== this.keys) { this.keys = k; this.bound = new Set(Object.values(k)); this.down.clear(); } }

  /** False while a menu is open: input is ignored, levels drop to zero and the pointer is released. */
  get enabled(): boolean { return this.on; }
  set enabled(v: boolean) {
    if (v && !this.on) this.pad.requireRelease();
    this.on = v; if (!v && this.locked) document.exitPointerLock();
  }

  attach(canvas: HTMLElement): void {
    this.canvas = canvas;
    window.addEventListener("keydown", (e) => this.onKeyDown(e));
    window.addEventListener("keyup", (e) => this.down.delete(e.code));
    window.addEventListener("blur", () => this.releaseAll());
    canvas.addEventListener("pointerdown", (e) => this.onPointerDown(e));
    window.addEventListener("pointerup", (e) => { this.mouseButtons = this.on ? e.buttons : 0; });
    document.addEventListener("mousemove", (e) => { if (this.locked && this.on) this.onLook?.(e.movementX, e.movementY); });
    document.addEventListener("pointerlockchange", () => this.onLockState());
    canvas.addEventListener("contextmenu", (e) => e.preventDefault());
    canvas.addEventListener("wheel", (e) => this.onWheel(e), { passive: false });
  }

  /** Non-repeating key press callback (B feed, R reset...). */
  onKeyPress(fn: (code: string) => void): void { this.pressHandler = fn; }

  get isLocked(): boolean { return this.locked; }

  /** Fills and returns the reused input: camera-relative move + crosshair aim. dt in seconds (pad look speed). */
  sample(camera: THREE.Camera, dt: number): HumanInput {
    const a = this.action;
    a.spin = 0;
    if (!this.on) {
      a.moveX = 0; a.moveZ = 0; a.buttons = 0;
      return a;
    }
    this.pad.poll(); if (this.pad.startPressed) this.onMenu?.();
    camera.getWorldDirection(this.fwd);
    this.move(a);
    a.buttons = this.buttons() | this.pad.buttons;
    this.latched = 0;
    this.aim(camera, a);
    if (this.pad.lookX !== 0 || this.pad.lookY !== 0) this.onLook?.(this.pad.lookX * PAD_LOOK_PX_PER_S * dt, this.pad.lookY * PAD_LOOK_PX_PER_S * dt);
    return a;
  }

  releaseAll(): void {
    this.down.clear();
    this.mouseButtons = 0;
    this.latched = 0;
  }

  private move(a: HumanInput): void {
    let fx = this.fwd.x;
    let fz = this.fwd.z;
    const len = Math.hypot(fx, fz);
    if (len < 1e-6) { fx = 1; fz = 0; } else { fx /= len; fz /= len; }
    const f = clampUnit(this.axis(this.keys.down, this.keys.up) + this.pad.forward);
    const r = clampUnit(this.axis(this.keys.left, this.keys.right) + this.pad.right);
    let mx = fx * f - fz * r; // right = (-fz, fx): yaw 0 -> D = +Z
    let mz = fz * f + fx * r;
    const m = Math.hypot(mx, mz);
    if (m > 1) { mx /= m; mz /= m; }
    a.moveX = mx; a.moveZ = mz;
  }

  private aim(camera: THREE.Camera, a: HumanInput): void {
    this.raycaster.setFromCamera(this.center, camera);
    const hit = this.raycaster.ray.intersectPlane(this.floor, this.hit);
    if (hit !== null && hit.x > RIVAL_MIN_X) {
      a.targetX = hit.x; // exact clamp to the rival half is the sim's job
      a.targetZ = hit.z;
      return;
    }
    if (hit !== null && hit.x < LOW_AIM_X) { a.targetX = hit.x; a.targetZ = hit.z; return; } // own half: no net assist
    const p = camera.position;
    const fx = this.fwd.x;
    const depth = hit === null ? DEEP_TARGET_X : SHORT_TARGET_X;
    if (fx < MIN_FORWARD_X) {
      a.targetX = depth;
      a.targetZ = Math.max(-COURT_HALF_Z, Math.min(COURT_HALF_Z, p.z));
      return;
    }
    const t = (depth - p.x) / fx; // forward ray projected onto the court
    a.targetX = depth;
    a.targetZ = Math.max(-COURT_HALF_Z, Math.min(COURT_HALF_Z, p.z + this.fwd.z * Math.max(0, t)));
  }

  private buttons(): number {
    let b = 0;
    const mouse = this.mouseButtons | this.latched;
    if (mouse & MOUSE_LEFT) b |= Btn.Strike;
    if (mouse & MOUSE_RIGHT) b |= Btn.Lob;
    if (this.down.has(this.keys.jump)) b |= Btn.Jump;
    if (this.down.has(this.keys.dash)) b |= Btn.Dash; // held level; the sim detects the press edge itself
    return b;
  }

  private axis(neg: string, pos: string): number {
    return (this.down.has(pos) ? 1 : 0) - (this.down.has(neg) ? 1 : 0);
  }

  private onKeyDown(e: KeyboardEvent): void {
    if (!this.on) return;
    if (this.bound.has(e.code)) e.preventDefault(); // Space scroll, Tab focus, arrows... stay in the game
    if (e.repeat) return;
    if (e.code === this.keys.free) { this.toggleLock(); return; }
    this.down.add(e.code);
    this.pressHandler?.(e.code);
  }

  private toggleLock(): void {
    if (!this.locked) { this.lock(); return; }
    this.freeRequested = true;
    document.exitPointerLock();
  }

  /** Requests pointer lock (needs a user gesture; failures are retried on the next click). */
  lock(): void {
    const c = this.canvas;
    if (!c || this.locked) return;
    const p: unknown = c.requestPointerLock();
    if (p instanceof Promise) p.catch(() => undefined); // denied (no gesture / cooldown): next click retries
  }

  private onPointerDown(e: PointerEvent): void {
    if (!this.on) { this.mouseButtons = 0; return; }
    if (!this.locked) this.lock(); // clicks still play while unlocked
    this.mouseButtons = e.buttons;
    this.latched |= e.buttons;
  }

  private onLockState(): void {
    this.locked = document.pointerLockElement === this.canvas;
    this.onLockChange?.(this.locked);
    if (this.locked) { this.freeRequested = false; return; }
    this.mouseButtons = 0;
    const asked = this.freeRequested;
    this.freeRequested = false;
    if (this.on && !asked) this.onLockLost?.();
  }

  private onWheel(e: WheelEvent): void {
    if (!this.on) return;
    e.preventDefault();
    this.onZoom?.(e.deltaY);
  }
}
