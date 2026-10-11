// Shared bits of the SVG HUD widgets (action bar, crosshair charge).
const SVG_NS = "http://www.w3.org/2000/svg";

/** Strike charge at or above this counts as full (snapshot charge is 0..1). */
export const FULL_CHARGE = 0.999;

export function svgNode<K extends keyof SVGElementTagNameMap>(tag: K, attrs: Readonly<Record<string, string>>): SVGElementTagNameMap[K] {
  const e = document.createElementNS(SVG_NS, tag);
  for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, v);
  return e;
}
