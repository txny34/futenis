// Strike charge around the crosshair (where the eyes are while aiming), like a
// bow draw in a shooter: an arc fills clockwise from 12 o'clock, glows and
// pulses at full charge, and bursts outward on release. Touches the DOM only on
// change (the arc is quantized). The burst is driven by the human's Kick event
// (not inferred from the charge dropping) and restarts by alternating classes.
import { FULL_CHARGE, svgNode } from "./hud-svg";

const ARC_R = 20;
const ARC_C = 2 * Math.PI * ARC_R;
const STEPS = 48;
const BURST_MS = 520;
const TICKS = "M0 -27 V-31 M27 0 H31 M0 27 V31 M-27 0 H-31"; // spin around the arc while charging


export class ChargeReticle {
  private readonly arc: SVGCircleElement;
  private last = -1;
  private wasFull = false;
  private burstUntil = 0;
  private flip = false; // alternates is-release-a/-b so back-to-back shots restart the burst

  constructor(private readonly root: HTMLElement) {
    const svg = svgNode("svg", { viewBox: "-32 -32 64 64", class: "charge", "aria-hidden": "true" });
    const ring = (cls: string): SVGCircleElement => svgNode("circle", { r: String(ARC_R), class: cls });
    this.arc = ring("charge__arc");
    this.arc.setAttribute("stroke-dasharray", ARC_C.toFixed(2));
    this.arc.setAttribute("stroke-dashoffset", ARC_C.toFixed(2));
    this.arc.setAttribute("transform", "rotate(-90)");
    const ticks = svgNode("g", { class: "charge__spin" });
    ticks.append(svgNode("path", { d: TICKS, class: "charge__ticks" }));
    svg.append(ring("charge__track"), ring("charge__burst"), ring("charge__burst charge__burst--late"), ticks, this.arc);
    root.append(svg);
  }

  /** charge 0..1 from the snapshot (fills the arc); nowMs ends the release burst. */
  update(charge: number, nowMs: number): void {
    const q = Math.round(Math.max(0, Math.min(1, charge)) * STEPS);
    if (q !== this.last) {
      this.last = q;
      this.arc.setAttribute("stroke-dashoffset", (ARC_C * (1 - q / STEPS)).toFixed(2));
      this.root.classList.toggle("is-charging", q > 0);
    }
    const full = charge >= FULL_CHARGE;
    if (full !== this.wasFull) {
      this.wasFull = full;
      this.root.classList.toggle("is-full", full);
    }
    if (this.burstUntil > 0 && nowMs > this.burstUntil) {
      this.burstUntil = 0;
      this.root.classList.remove("is-release-a", "is-release-b");
    }
  }

  /** The human kicked the ball (Kick event): play the release burst. */
  release(nowMs: number): void {
    this.flip = !this.flip;
    this.root.classList.remove(this.flip ? "is-release-b" : "is-release-a");
    this.root.classList.add(this.flip ? "is-release-a" : "is-release-b");
    this.burstUntil = nowMs + BURST_MS;
  }
}
