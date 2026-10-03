import type { PlayerAction } from "./engine";

// Camera sits behind side 0 looking along +X (Y up). Screen right = forward x up
// = (1,0,0) x (0,1,0) = (0,0,1) = +Z; screen up (into the court) = +X.
// Both players use pure screen space: up=+X, down=-X, right=+Z, left=-Z.
interface Bindings {
  readonly up: string;
  readonly down: string;
  readonly left: string;
  readonly right: string;
  readonly kick: string;
  readonly lob: string;
}

export type Side = 0 | 1;

const BINDINGS: readonly [Bindings, Bindings] = [
  { up: "KeyW", down: "KeyS", left: "KeyA", right: "KeyD", kick: "KeyF", lob: "KeyG" },
  { up: "ArrowUp", down: "ArrowDown", left: "ArrowLeft", right: "ArrowRight", kick: "KeyK", lob: "KeyL" },
];
const SIDES: readonly Side[] = [0, 1];
const CHARGE_TIME_S = 0.6;
const MS_PER_S = 1000;
const AIM_FORWARD = 1; // sim forces the shot toward the rival side
const LOB_LOFT = 1;
const FLAT_LOFT = 0;
const PREVENT_DEFAULT = new Set(["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight", "Space"]);

const emptyAction = (): PlayerAction => ({ moveX: 0, moveZ: 0, kick: false, aimX: AIM_FORWARD, aimZ: 0, loft: 0, power: 0 });

/** Keyboard -> PlayerAction. Knows nothing about rendering or rules. */
export class Keyboard {
  private readonly down = new Set<string>();
  private readonly chargeStart: [number, number] = [-1, -1]; // -1 = not charging
  private readonly kickPending: [boolean, boolean] = [false, false];
  private readonly actions: [PlayerAction, PlayerAction] = [emptyAction(), emptyAction()];
  private pressHandler: ((code: string) => void) | null = null;

  attach(target: Window): void {
    target.addEventListener("keydown", (e) => this.handleDown(e));
    target.addEventListener("keyup", (e) => this.handleUp(e));
    target.addEventListener("blur", () => this.down.clear());
  }

  /** Non-repeating key press callback (e.g. R to reset). */
  onKeyPress(fn: (code: string) => void): void {
    this.pressHandler = fn;
  }

  /** 0..1 charge of a held kick, for UI. */
  charge(side: Side, nowMs: number): number {
    const start = this.chargeStart[side];
    return start < 0 ? 0 : Math.min(1, (nowMs - start) / MS_PER_S / CHARGE_TIME_S);
  }

  /** Fills and returns the reused action for a side. Call clearKicks() after stepping. */
  action(side: Side): PlayerAction {
    const a = this.actions[side];
    const b = BINDINGS[side];
    a.moveX = this.axis(b.down, b.up);
    a.moveZ = this.axis(b.left, b.right);
    a.kick = this.kickPending[side];
    if (!a.kick) a.aimZ = a.moveZ;
    return a;
  }

  /** Kicks are edge-triggered: send once, then clear. */
  clearKicks(): void {
    this.kickPending[0] = false;
    this.kickPending[1] = false;
  }

  private axis(neg: string, pos: string): number {
    return (this.down.has(pos) ? 1 : 0) - (this.down.has(neg) ? 1 : 0);
  }

  private handleDown(e: KeyboardEvent): void {
    if (PREVENT_DEFAULT.has(e.code)) e.preventDefault();
    if (e.repeat) return;
    this.down.add(e.code);
    for (const side of SIDES) {
      if (e.code === BINDINGS[side].kick) this.chargeStart[side] = performance.now();
    }
    this.pressHandler?.(e.code);
  }

  private handleUp(e: KeyboardEvent): void {
    if (PREVENT_DEFAULT.has(e.code)) e.preventDefault();
    for (const side of SIDES) {
      if (e.code === BINDINGS[side].kick && this.chargeStart[side] >= 0) this.releaseKick(side);
    }
    this.down.delete(e.code);
  }

  private releaseKick(side: Side): void {
    const b = BINDINGS[side];
    const a = this.actions[side];
    a.power = this.charge(side, performance.now());
    a.loft = this.down.has(b.lob) ? LOB_LOFT : FLAT_LOFT;
    a.aimZ = this.axis(b.left, b.right); // screen left/right
    this.kickPending[side] = true;
    this.chargeStart[side] = -1;
  }
}
