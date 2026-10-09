import { Btn } from "./snapshot";

// Standard-mapping gamepad (Xbox/PlayStation layout), read once per sample.
// Left stick = move (camera-relative, like WASD), right stick = camera look,
// RT/X = strike, LT/B = lob, A = jump, LB/RB = dash. Held levels; the sim
// detects edges itself.
const AXIS_MOVE_X = 0;
const AXIS_MOVE_Y = 1;
const AXIS_LOOK_X = 2;
const AXIS_LOOK_Y = 3;
const BTN_JUMP = 0; // A
const BTN_LOB_SOFT = 1; // B
const BTN_STRIKE_SOFT = 2; // X
const BTN_DASH_LB = 4;
const BTN_DASH_RB = 5;
const BTN_LOB_TRIGGER = 6; // LT
const BTN_STRIKE_TRIGGER = 7; // RT
const TRIGGER_THRESHOLD = 0.5;
const STICK_DEADZONE = 0.15;
const STICK_LIVE = 1 - STICK_DEADZONE;

/** Clamps v to [-1, 1]. */
export function clampUnit(v: number): number {
  return v < -1 ? -1 : v > 1 ? 1 : v;
}

/** Radial deadzone on a single stick axis: scales the remaining travel to 0..1. */
function applyDeadzone(v: number, magnitude: number): number {
  if (magnitude < STICK_DEADZONE) return 0;
  const scale = (magnitude - STICK_DEADZONE) / STICK_LIVE / magnitude;
  return clampUnit(v * scale);
}

/** First connected gamepad -> move/look levels + buttons. Allocation-free per poll. */
export class GamepadReader {
  forward = 0;
  right = 0;
  lookX = 0;
  lookY = 0;
  buttons = 0;
  connected = false;

  poll(): void {
    const pads = navigator.getGamepads?.() ?? [];
    const pad = this.pick(pads);
    if (!pad) {
      this.forward = 0; this.right = 0; this.lookX = 0; this.lookY = 0; this.buttons = 0;
      this.connected = false;
      return;
    }
    this.connected = true;
    this.readSticks(pad);
    this.buttons = this.readButtons(pad);
  }

  private pick(pads: readonly (Gamepad | null)[]): Gamepad | null {
    let fallback: Gamepad | null = null;
    for (const pad of pads) {
      if (!pad) continue;
      if (pad.mapping === "standard") return pad;
      if (!fallback) fallback = pad;
    }
    return fallback;
  }

  private readSticks(pad: Gamepad): void {
    const mx = pad.axes[AXIS_MOVE_X] ?? 0;
    const my = pad.axes[AXIS_MOVE_Y] ?? 0;
    const moveMag = Math.hypot(mx, my);
    this.forward = -applyDeadzone(my, moveMag);
    this.right = applyDeadzone(mx, moveMag);
    const lx = pad.axes[AXIS_LOOK_X] ?? 0;
    const ly = pad.axes[AXIS_LOOK_Y] ?? 0;
    const lookMag = Math.hypot(lx, ly);
    this.lookX = applyDeadzone(lx, lookMag);
    this.lookY = applyDeadzone(ly, lookMag);
  }

  private readButtons(pad: Gamepad): number {
    const b = pad.buttons;
    let out = 0;
    if ((b[BTN_STRIKE_TRIGGER]?.value ?? 0) > TRIGGER_THRESHOLD || b[BTN_STRIKE_SOFT]?.pressed) out |= Btn.Strike;
    if ((b[BTN_LOB_TRIGGER]?.value ?? 0) > TRIGGER_THRESHOLD || b[BTN_LOB_SOFT]?.pressed) out |= Btn.Lob;
    if (b[BTN_JUMP]?.pressed) out |= Btn.Jump;
    if (b[BTN_DASH_LB]?.pressed || b[BTN_DASH_RB]?.pressed) out |= Btn.Dash;
    return out;
  }
}
