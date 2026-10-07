// Procedural card art for "Elegí un modo": a padel court in perspective, glass
// walls, net, foot-padel players (no rackets) and the ball's arc. Inline SVG, colors
// from CSS classes (styles/play-modal.css -> --fx-* tokens). Built once per card.
const NS = "http://www.w3.org/2000/svg";
type Attrs = Readonly<Record<string, string | number>>;

function shape(tag: string, cls: string, a: Attrs): SVGElement {
  const e = document.createElementNS(NS, tag);
  e.setAttribute("class", cls);
  for (const [k, v] of Object.entries(a)) e.setAttribute(k, String(v));
  return e;
}

/** One player: head, torso and a kicking leg (mirrored for the far team). */
function player(x: number, y: number, s: number, far: boolean): SVGElement {
  const g = shape("g", far ? "fx-art__rival" : "fx-art__home", { transform: `translate(${x} ${y}) scale(${far ? -s : s} ${s})` });
  g.append(
    shape("circle", "", { cx: 0, cy: -46, r: 9 }),
    shape("path", "", { d: "M-9 -34h18l3 26h-24z" }),
    shape("path", "fx-art__limb", { d: "M-5 -8l-4 22M5 -8l14 12" }),
    shape("path", "fx-art__limb", { d: "M-9 -30l-10 14M9 -30l9 12" }),
  );
  return g;
}

function court(svg: SVGElement): void {
  svg.append(
    shape("rect", "fx-art__glass", { x: 20, y: 18, width: 280, height: 60, rx: 4 }),
    shape("polygon", "fx-art__floor", { points: "40,78 280,78 318,160 2,160" }),
    shape("path", "fx-art__lines", { d: "M160 78V160M70 98H250M30 132H290" }),
    shape("path", "fx-art__net", { d: "M24 112H296" }),
    shape("rect", "fx-art__netband", { x: 24, y: 104, width: 272, height: 8 }),
  );
}

/** members: players per team (1 or 2). Home team in front, rivals beyond the net. */
export function modeArt(members: number): SVGSVGElement {
  const svg = document.createElementNS(NS, "svg");
  svg.setAttribute("viewBox", "0 0 320 160");
  svg.setAttribute("preserveAspectRatio", "xMidYMid slice");
  svg.setAttribute("aria-hidden", "true");
  svg.setAttribute("class", "fx-art");
  court(svg);
  const gap = members === 1 ? 0 : 70;
  for (let i = 0; i < members; i++) svg.append(player(160 + (i - (members - 1) / 2) * gap, 98, 0.7, true));
  svg.append(
    shape("path", "fx-art__arc", { d: "M120 140Q170 30 205 80" }),
    shape("circle", "fx-art__ball", { cx: 205, cy: 80, r: 6 }),
  );
  for (let i = 0; i < members; i++) svg.append(player(160 + (i - (members - 1) / 2) * gap * 1.6, 150, 1.1, false));
  return svg;
}
