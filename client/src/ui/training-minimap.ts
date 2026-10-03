/** Top-down court minimap for the Entrenamiento panel: machine zone (rival half) + bounce zone (your half). */
import type { TrainingSettings } from "./training-settings";

// The player looks from below: X = +10 (rival back wall) at the top, X = -10 at
// the bottom; Z = -5 (Izquierda) on the left. 1 m = 10 SVG units.
const SVG_NS = "http://www.w3.org/2000/svg";
const UNIT = 10;
const HALF_LEN = 10;
const HALF_WID = 5;
const SERVICE_LINE = 6.95;
const W = 2 * HALF_WID * UNIT;
const H = 2 * HALF_LEN * UNIT;

const sx = (z: number): number => (z + HALF_WID) * UNIT;
const sy = (x: number): number => (HALF_LEN - x) * UNIT;

function svg<K extends keyof SVGElementTagNameMap>(tag: K, cls: string, attrs: Record<string, number | string>): SVGElementTagNameMap[K] {
  const e = document.createElementNS(SVG_NS, tag);
  e.setAttribute("class", cls);
  for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, String(v));
  return e;
}

function line(x1: number, y1: number, x2: number, y2: number, cls = "fx-map__line"): SVGLineElement {
  return svg("line", cls, { x1, y1, x2, y2 });
}

function setRect(r: SVGRectElement, zLo: number, zHi: number, xNear: number, xFar: number): void {
  const top = Math.min(sy(xNear), sy(xFar));
  r.setAttribute("x", String(sx(zLo)));
  r.setAttribute("y", String(top));
  r.setAttribute("width", String(Math.max(1, (zHi - zLo) * UNIT)));
  r.setAttribute("height", String(Math.max(1, Math.abs(xFar - xNear) * UNIT)));
}

export interface Minimap {
  readonly root: SVGSVGElement;
  update(t: TrainingSettings): void;
}

export function minimap(): Minimap {
  const root = svg("svg", "fx-map", { viewBox: `-4 -4 ${W + 8} ${H + 8}`, role: "img", "aria-label": "Cancha: zona de máquina y zona de pique" });
  root.append(
    svg("rect", "fx-map__court", { x: 0, y: 0, width: W, height: H, rx: 2 }),
    line(0, sy(SERVICE_LINE), W, sy(SERVICE_LINE)),
    line(0, sy(-SERVICE_LINE), W, sy(-SERVICE_LINE)),
    line(W / 2, sy(SERVICE_LINE), W / 2, sy(-SERVICE_LINE)),
  );
  const machine = svg("rect", "fx-map__machine", { rx: 2 });
  const bounce = svg("rect", "fx-map__bounce", { rx: 2 });
  root.append(machine, bounce, line(-4, H / 2, W + 4, H / 2, "fx-map__net"));
  return {
    root,
    update(t: TrainingSettings): void {
      setRect(machine, t.machineZ.lo, t.machineZ.hi, t.machineX.lo, t.machineX.hi);
      setRect(bounce, t.feedZ.lo, t.feedZ.hi, -t.feedX.lo, -t.feedX.hi);
    },
  };
}
