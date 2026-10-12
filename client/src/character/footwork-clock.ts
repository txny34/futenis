import { Fidget } from "./footwork";

// Timers behind the footwork layers that only the animator can see: a skid
// after a hard brake, and idle fidgets every few seconds of standing still.
// Visual only, driven by speed and dt; allocation free.

const SKID_FROM = 0.55; // smoothed normalized speed a stop must start above
const SKID_DECEL = 3; // smoothed normalized speed lost per second to count as a brake
const SKID_MOVING = 0.03; // a brake still slides: an instant 0 is a teleport or a point end
const SPEED_HZ = 12; // speed smoothing: snapshot velocity steps per sim/net tick, not per frame
const SKID_S = 0.4;
const FIDGET_AFTER_S = 4.5; // of standing still
const FIDGET_S = 2.4;
const FIDGET_FADE_HZ = 6;
const STILL_SPEED = 0.05;

export class FootworkClock {
  skid = 0; // 0..1 weight
  fidget: Fidget = "look";
  fidgetU = 0; // 0..1 progress (kept while fading out)
  fidgetW = 0; // 0..1 weight
  private smooth = 0;
  private skidLeft = 0;
  private still = 0;
  private count = 0;

  update(speed: number, grounded: boolean, busy: boolean, dt: number): void {
    this.updateSkid(speed, grounded, dt);
    this.still = grounded && !busy && speed < STILL_SPEED ? this.still + dt : 0;
    const on = this.still >= FIDGET_AFTER_S;
    this.fidgetW += ((on ? 1 : 0) - this.fidgetW) * Math.min(1, dt * FIDGET_FADE_HZ);
    if (!on) return;
    const u = (this.still - FIDGET_AFTER_S) / FIDGET_S;
    if (u < 1) { this.fidgetU = u; return; }
    this.still = 0; // wait again, then the other fidget
    this.fidget = ++this.count % 2 === 0 ? "look" : "stretch";
  }

  private updateSkid(speed: number, grounded: boolean, dt: number): void {
    const prev = this.smooth;
    this.smooth += (speed - prev) * Math.min(1, dt * SPEED_HZ);
    const decel = dt > 0 ? (prev - this.smooth) / dt : 0;
    if (grounded && prev > SKID_FROM && decel > SKID_DECEL && speed > SKID_MOVING) this.skidLeft = SKID_S;
    this.skidLeft = Math.max(0, this.skidLeft - dt);
    const s = this.skidLeft / SKID_S;
    this.skid = s * s * (3 - 2 * s);
  }
}
