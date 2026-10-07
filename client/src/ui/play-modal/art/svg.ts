// Tiny SVG builders for the play-modal card art. Colors come from CSS classes
// (styles/play-cards.css -> --fx-* tokens), never from literals here.
export const NS = "http://www.w3.org/2000/svg";
export type Attrs = Readonly<Record<string, string | number>>;
export type Pt = readonly [number, number];
/** Gradient stop: offset (0..1) + class that sets stop-color/stop-opacity. */
export type Stop = readonly [number, string];

export function shape(tag: string, cls: string, a: Attrs = {}): SVGElement {
  const e = document.createElementNS(NS, tag);
  if (cls !== "") e.setAttribute("class", cls);
  for (const [k, v] of Object.entries(a)) e.setAttribute(k, String(v));
  return e;
}

export function group(cls: string, a: Attrs, kids: readonly SVGElement[]): SVGElement {
  const g = shape("g", cls, a);
  g.append(...kids);
  return g;
}

export function gradient(id: string, radial: boolean, a: Attrs, stops: readonly Stop[]): SVGElement {
  const g = shape(radial ? "radialGradient" : "linearGradient", "", { id, ...a });
  for (const [o, c] of stops) g.append(shape("stop", c, { offset: o }));
  return g;
}

export const VERTICAL: Attrs = { x1: 0, y1: 0, x2: 0, y2: 1 };
export const HORIZONTAL: Attrs = { x1: 0, y1: 0, x2: 1, y2: 0 };
export const points = (p: readonly Pt[]): string => p.map(([x, y]) => `${x},${y}`).join(" ");

/** Root <svg> for a decorative graphic. */
export function svgRoot(cls: string, viewBox: string, slice: boolean): SVGSVGElement {
  const svg = document.createElementNS(NS, "svg");
  svg.setAttribute("viewBox", viewBox);
  svg.setAttribute("preserveAspectRatio", slice ? "xMidYMid slice" : "xMidYMid meet");
  svg.setAttribute("aria-hidden", "true");
  svg.setAttribute("class", cls);
  return svg;
}
