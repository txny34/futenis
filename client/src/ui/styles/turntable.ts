// Drag-to-spin for the Estilos preview: a full 360° turn by dragging (mouse,
// touch or pen), a slow idle spin when left alone, and a turn to a given yaw
// (the back, to show the shirt number). Pure state: the preview reads `yaw`.

const DRAG_RAD_PER_PX = 0.012;
const IDLE_SPIN = 0.35; // rad/s once nobody touched it for a while
const IDLE_AFTER_S = 2.5;
const SEEK_HZ = 5;
const TAU = Math.PI * 2;

const wrap = (a: number): number => Math.atan2(Math.sin(a), Math.cos(a));

export class Turntable {
  yaw = 0;
  private dragging = false;
  private lastX = 0;
  private idle = IDLE_AFTER_S; // seconds since the last drag
  private seek: number | null = null;

  constructor(el: HTMLElement) {
    el.style.touchAction = "none"; // a drag spins, it never scrolls the page
    el.style.cursor = "grab";
    el.addEventListener("pointerdown", (e) => {
      this.dragging = true;
      this.lastX = e.clientX;
      this.seek = null;
      el.setPointerCapture(e.pointerId);
      el.style.cursor = "grabbing";
    });
    el.addEventListener("pointermove", (e) => {
      if (!this.dragging) return;
      this.yaw = (this.yaw + (e.clientX - this.lastX) * DRAG_RAD_PER_PX) % TAU;
      this.lastX = e.clientX;
      this.idle = 0;
    });
    const end = (): void => { this.dragging = false; el.style.cursor = "grab"; };
    el.addEventListener("pointerup", end);
    el.addEventListener("pointercancel", end);
  }

  /** Turn to yaw (e.g. Math.PI: the back) the shortest way, then hold there a moment. */
  turnTo(yaw: number): void {
    this.seek = yaw;
    this.idle = 0;
  }

  update(dt: number): void {
    if (this.dragging) return;
    this.idle += dt;
    if (this.seek !== null) {
      const d = wrap(this.seek - this.yaw);
      this.yaw += d * Math.min(1, dt * SEEK_HZ);
      if (Math.abs(d) < 0.01) this.seek = null;
      return;
    }
    if (this.idle > IDLE_AFTER_S) this.yaw = (this.yaw + dt * IDLE_SPIN) % TAU;
  }
}
