import * as THREE from "three";
import type { HumanInput } from "./engine";
import { Btn } from "./snapshot";

// Single human on slot 0. Camera sits behind team 0 looking along +X, so
// screen up = +X and screen right = +Z: W=+X, S=-X, D=+Z, A=-Z.
// Everything is a held LEVEL (PlayerInput v2); the sim detects edges itself.
const KEY_UP = "KeyW";
const KEY_DOWN = "KeyS";
const KEY_LEFT = "KeyA";
const KEY_RIGHT = "KeyD";
const KEY_JUMP = "Space";
const SPRINT_KEYS = ["ShiftLeft", "ShiftRight"] as const;
const PREVENT_DEFAULT = new Set(["Space", "ArrowUp", "ArrowDown"]);
const MOUSE_LEFT = 1; // PointerEvent.buttons bit
const MOUSE_RIGHT = 2;
const SPIN_MIN = -1;
const SPIN_MAX = 1;
const DEFAULT_TARGET_X = 5; // middle of the rival half until the mouse moves

/** Keyboard + mouse -> HumanInput. Knows nothing about rules. Allocation-free per frame. */
export class Controls {
  /** False while a menu is open: pointer/keys are ignored and levels drop to zero. */
  enabled = false;
  private readonly down = new Set<string>();
  private mouseButtons = 0;
  private spin = 0;
  private hasPointer = false;
  private readonly ndc = new THREE.Vector2();
  private readonly raycaster = new THREE.Raycaster();
  private readonly floor = new THREE.Plane(new THREE.Vector3(0, 1, 0), 0);
  private readonly hit = new THREE.Vector3();
  private readonly action: HumanInput = { moveX: 0, moveZ: 0, targetX: DEFAULT_TARGET_X, targetZ: 0, buttons: 0, spin: 0 };
  private pressHandler: ((code: string) => void) | null = null;

  attach(canvas: HTMLElement): void {
    window.addEventListener("keydown", (e) => this.onKeyDown(e));
    window.addEventListener("keyup", (e) => this.down.delete(e.code));
    window.addEventListener("blur", () => this.releaseAll());
    canvas.addEventListener("pointermove", (e) => this.onPointer(e, canvas));
    canvas.addEventListener("pointerdown", (e) => this.onPointer(e, canvas));
    window.addEventListener("pointerup", (e) => { this.mouseButtons = e.buttons; });
    canvas.addEventListener("contextmenu", (e) => e.preventDefault());
    canvas.addEventListener("wheel", (e) => this.onWheel(e), { passive: false });
  }

  /** Non-repeating key press callback (B respawn, R reset...). */
  onKeyPress(fn: (code: string) => void): void { this.pressHandler = fn; }

  /** Current wheel spin level -1/0/+1, for the HUD. */
  get spinLevel(): number { return this.spin; }

  /** Fills and returns the reused input, projecting the cursor onto the court plane. */
  sample(camera: THREE.Camera): HumanInput {
    const a = this.action;
    if (!this.enabled) {
      a.moveX = 0; a.moveZ = 0; a.buttons = 0;
      return a;
    }
    a.moveX = this.axis(KEY_DOWN, KEY_UP);
    a.moveZ = this.axis(KEY_LEFT, KEY_RIGHT);
    a.buttons = this.buttons();
    a.spin = this.spin;
    if (this.hasPointer) this.project(camera, a);
    return a;
  }

  releaseAll(): void {
    this.down.clear();
    this.mouseButtons = 0;
  }

  private project(camera: THREE.Camera, a: HumanInput): void {
    this.raycaster.setFromCamera(this.ndc, camera);
    if (this.raycaster.ray.intersectPlane(this.floor, this.hit) === null) return;
    a.targetX = this.hit.x; // clamp to the rival half is the sim's job
    a.targetZ = this.hit.z;
  }

  private buttons(): number {
    let b = 0;
    if (this.mouseButtons & MOUSE_LEFT) b |= Btn.Strike;
    if (this.mouseButtons & MOUSE_RIGHT) b |= Btn.Lob;
    if (this.down.has(KEY_JUMP)) b |= Btn.Jump;
    if (this.down.has(SPRINT_KEYS[0]) || this.down.has(SPRINT_KEYS[1])) b |= Btn.Sprint;
    return b;
  }

  private axis(neg: string, pos: string): number {
    return (this.down.has(pos) ? 1 : 0) - (this.down.has(neg) ? 1 : 0);
  }

  private onKeyDown(e: KeyboardEvent): void {
    if (!this.enabled) return;
    if (PREVENT_DEFAULT.has(e.code)) e.preventDefault();
    if (e.repeat) return;
    this.down.add(e.code);
    this.pressHandler?.(e.code);
  }

  private onPointer(e: PointerEvent, canvas: HTMLElement): void {
    // offsetX/Y are relative to the canvas (the event target): no DOMRect allocation.
    const w = canvas.clientWidth || 1;
    const h = canvas.clientHeight || 1;
    this.ndc.set((e.offsetX / w) * 2 - 1, -(e.offsetY / h) * 2 + 1);
    this.hasPointer = true;
    this.mouseButtons = this.enabled ? e.buttons : 0;
  }

  private onWheel(e: WheelEvent): void {
    if (!this.enabled) return;
    e.preventDefault();
    const dir = e.deltaY < 0 ? 1 : e.deltaY > 0 ? -1 : 0; // wheel up = topspin
    this.spin = Math.max(SPIN_MIN, Math.min(SPIN_MAX, this.spin + dir));
  }
}
