// Strike charge around the crosshair (where the eyes are while aiming), like a
// bow draw in a shooter: an arc fills clockwise from 12 o'clock, glows and
// pulses at full charge, and bursts outward on release. Touches the DOM only on
// change (the arc is quantized) and restarts the burst by toggling a class.
const SVG_NS = "http://www.w3.org/2000/svg";
const ARC_R = 20;
const ARC_C = 2 * Math.PI * ARC_R;
const STEPS = 48;
const FULL = 0.999;
const BURST_MS = 380;

function node<K extends keyof SVGElementTagNameMap>(tag: K, attrs: Readonly<Record<string, string>>): SVGElementTagNameMap[K] {
  const e = document.createElementNS(SVG_NS, tag);
  for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, v);
  return e;
}

export class ChargeReticle {
  private readonly arc: SVGCircleElement;
  private last = -1;
  private wasFull = false;
  private burstUntil = 0;

  constructor(private readonly root: HTMLElement) {
    const svg = node("svg", { viewBox: "-32 -32 64 64", class: "charge", "aria-hidden": "true" });
    const ring = (cls: string): SVGCircleElement => node("circle", { r: String(ARC_R), class: cls });
    this.arc = ring("charge__arc");
    this.arc.setAttribute("stroke-dasharray", ARC_C.toFixed(2));
    this.arc.setAttribute("stroke-dashoffset", ARC_C.toFixed(2));
    this.arc.setAttribute("transform", "rotate(-90)");
    svg.append(ring("charge__track"), ring("charge__burst"), this.arc);
    root.append(svg);
  }

  /** charge 0..1 from the snapshot; nowMs drives the release burst. */
  update(charge: number, nowMs: number): void {
    const q = Math.round(Math.max(0, Math.min(1, charge)) * STEPS);
    if (q !== this.last) {
      if (q === 0 && this.last > 0) this.release(nowMs);
      this.last = q;
      this.arc.setAttribute("stroke-dashoffset", (ARC_C * (1 - q / STEPS)).toFixed(2));
      this.root.classList.toggle("is-charging", q > 0);
    }
    const full = charge >= FULL;
    if (full !== this.wasFull) {
      this.wasFull = full;
      this.root.classList.toggle("is-full", full);
    }
    if (this.burstUntil > 0 && nowMs > this.burstUntil) {
      this.burstUntil = 0;
      this.root.classList.remove("is-release");
    }
  }

  private release(nowMs: number): void {
    this.root.classList.remove("is-release");
    void this.root.offsetWidth; // restart the CSS animation on back-to-back shots
    this.root.classList.add("is-release");
    this.burstUntil = nowMs + BURST_MS;
  }
}
