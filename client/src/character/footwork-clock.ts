import { Fidget } from "./footwork";

// Timers behind the footwork layers that only the animator can see: a skid
// after a hard brake, and idle fidgets every few seconds of standing still.
// Visual only, driven by speed and dt; allocation free.

const SKID_FROM = 0.55; // normalized speed a stop must start above
const SKID_DECEL = 3.5; // normalized speed lost per second to count as a brake
const SKID_S = 0.4;
const FIDGET_AFTER_S = 4.5; // of standing still
const FIDGET_S = 2.4;
const STILL_SPEED = 0.05;

export class FootworkClock {
  skid = 0; // 0..1 weight
  fidget: Fidget = "look";
  fidgetU = 0; // 0..1 progress, 0 = none
  private prevSpeed = 0;
  private skidLeft = 0;
  private still = 0;
  private count = 0;

  update(speed: number, grounded: boolean, busy: boolean, dt: number): void {
    const decel = dt > 0 ? (this.prevSpeed - speed) / dt : 0;
    if (grounded && this.prevSpeed > SKID_FROM && decel > SKID_DECEL) this.skidLeft = SKID_S;
    this.prevSpeed = speed;
    this.skidLeft = Math.max(0, this.skidLeft - dt);
    const s = this.skidLeft / SKID_S;
    this.skid = s * s * (3 - 2 * s);
    this.still = grounded && !busy && speed < STILL_SPEED ? this.still + dt : 0;
    if (this.still < FIDGET_AFTER_S) { this.fidgetU = 0; return; }
    const u = (this.still - FIDGET_AFTER_S) / FIDGET_S;
    if (u >= 1) {
      this.still = 0; // wait again, then the other fidget
      this.fidget = ++this.count % 2 === 0 ? "look" : "stretch";
      this.fidgetU = 0;
      return;
    }
    this.fidgetU = u;
  }
}
