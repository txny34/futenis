import { gradient, HORIZONTAL, shape, svgRoot, VERTICAL } from "./svg";

// Golden trophy emblem (ranked cards + "Clasificatorias" heading) and the teal ball
// icon (casual heading). Gradient ids are prefixed per instance to stay unique.
const GOLD = [[0, "fx-st--gold-light"], [0.45, "fx-st--gold"], [1, "fx-st--gold-deep"]] as const;

export function trophyIcon(id: string, cls = "fx-trophy"): SVGSVGElement {
  const svg = svgRoot(cls, "0 0 64 64", false);
  const fill = `url(#${id}-g)`;
  const defs = shape("defs", "");
  defs.append(gradient(`${id}-g`, false, HORIZONTAL, GOLD), gradient(`${id}-b`, false, VERTICAL, GOLD));
  svg.append(
    defs,
    shape("path", "fx-trophy__handle", { d: "M18 13H9c0 13 6 19 14 20M46 13h9c0 13-6 19-14 20" }),
    shape("path", "", { d: "M16 8h32v12c0 12-8 18-16 18s-16-6-16-18z", fill }),
    shape("rect", "fx-trophy__rim", { x: 14.5, y: 5.5, width: 35, height: 4.5, rx: 2 }),
    shape("path", "", { d: "M29 37h6l2.5 9h-11z", fill }),
    shape("rect", "", { x: 22, y: 45.5, width: 20, height: 5, rx: 1.5, fill }),
    shape("rect", "", { x: 17, y: 50.5, width: 30, height: 8, rx: 2, fill: `url(#${id}-b)` }),
    shape("rect", "fx-trophy__plate", { x: 25, y: 52.8, width: 14, height: 3.4, rx: 1 }),
    shape("path", "fx-trophy__star", { d: "M32 14.5l2.3 4.7 5.2.7-3.8 3.6.9 5.1-4.6-2.4-4.6 2.4.9-5.1-3.8-3.6 5.2-.7z" }),
    shape("path", "fx-trophy__shine", { d: "M21 12c0 11 2.5 18 7 21.5" }),
  );
  return svg;
}

export function ballIcon(): SVGSVGElement {
  const svg = svgRoot("fx-ballicon", "0 0 24 24", false);
  svg.append(
    shape("circle", "fx-ballicon__body", { cx: 12, cy: 12, r: 10 }),
    shape("path", "fx-ballicon__seam", { d: "M4.5 5.5c4 3 4 10 0 13M19.5 5.5c-4 3-4 10 0 13" }),
  );
  return svg;
}
