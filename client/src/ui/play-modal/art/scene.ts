import { gradient, group, points, Pt, shape, VERTICAL } from "./svg";

// The arena behind the athletes: lit stands with a faint crowd, a padel court in
// perspective (glass walls with highlights, fence mesh, floor lines) and the net.
// Ranked scenes are at night: warm sky, gold glows and a spotlight on the court.
// viewBox 0 0 320 160. The back wall spans x 40..280, the floor widens towards us.
const BACK_Y = 78;
const TOP_Y = 44;
const NEAR_Y = 164;
const SPREAD = 46 / (NEAR_Y - BACK_Y);
const NET_Y = 112;
const NET_H = 15;
const LIGHTS = [40, 120, 200, 280] as const;

const leftX = (y: number): number => 40 - (y - BACK_Y) * SPREAD;
const rightX = (y: number): number => 320 - leftX(y);

export function sceneDefs(id: string, ranked: boolean): SVGElement {
  const t = ranked ? "gold" : "teal";
  const d = shape("defs", "");
  d.append(
    gradient(`${id}-sky`, false, VERTICAL, ranked ? [[0, "fx-st--night"], [1, "fx-st--ember"]] : [[0, "fx-st--deep"], [1, "fx-st--court"]]),
    gradient(`${id}-glow`, true, {}, [[0, `fx-st--${t}-hi`], [1, `fx-st--${t}-0`]]),
    gradient(`${id}-floor`, false, VERTICAL, [[0, "fx-st--floor-far"], [1, "fx-st--floor-near"]]),
    gradient(`${id}-beam`, false, VERTICAL, [[0, "fx-st--gold-hi"], [1, "fx-st--gold-0"]]),
  );
  return d;
}

function crowd(): SVGElement[] {
  const rows: string[] = ["", "", "", ""];
  for (let r = 0; r < 4; r++) {
    for (let c = 0; c < 40; c++) rows[(c * 7 + r * 3) % 4] += `M${4 + c * 8 + (r % 2) * 4} ${25 + r * 5.5}h0`;
  }
  return rows.map((d, k) => shape("path", `fx-art__fan fx-art__fan--${k}`, { d }));
}

export function stands(id: string, ranked: boolean): SVGElement {
  const kids: SVGElement[] = [
    shape("rect", "", { width: 320, height: 160, fill: `url(#${id}-sky)` }),
    shape("rect", "fx-art__stand", { y: 20, width: 320, height: 26 }),
    ...crowd(),
  ];
  for (const x of LIGHTS) {
    kids.push(shape("circle", "", { cx: x, cy: 8, r: 38, fill: `url(#${id}-glow)` }), shape("rect", "fx-art__lamp", { x: x - 7, y: 5, width: 14, height: 4, rx: 2 }));
  }
  if (ranked) kids.push(shape("polygon", "", { points: "146,0 174,0 262,160 58,160", fill: `url(#${id}-beam)` }));
  return group("", {}, kids);
}

function glassPanels(): SVGElement[] {
  const seams: string[] = [];
  const shine: string[] = [];
  for (let i = 0; i <= 5; i++) seams.push(`M${40 + i * 48} ${TOP_Y}V${BACK_Y}`);
  for (let i = 0; i < 5; i++) shine.push(`M${50 + i * 48} ${BACK_Y - 6}l14 -26M${58 + i * 48} ${BACK_Y - 4}l6 -11`);
  for (const a of [0.35, 0.75]) { // side-wall seams follow the slant
    seams.push(`M${40 - a * 46} ${TOP_Y + a * 52}V${BACK_Y + a * 86}`, `M${280 + a * 46} ${TOP_Y + a * 52}V${BACK_Y + a * 86}`);
  }
  return [shape("path", "fx-art__seam", { d: seams.join("") }), shape("path", "fx-art__shine", { d: shine.join("") })];
}

function mesh(): SVGElement {
  let d = "";
  for (let x = 40; x <= 280; x += 6) d += `M${x} 34l6 10M${x + 6} 34l-6 10`;
  return shape("path", "fx-art__mesh", { d });
}

export function court(id: string): SVGElement {
  const floor: readonly Pt[] = [[40, BACK_Y], [280, BACK_Y], [rightX(NEAR_Y), NEAR_Y], [leftX(NEAR_Y), NEAR_Y]];
  const svc = (y: number): string => `M${leftX(y)} ${y}H${rightX(y)}`;
  return group("", {}, [
    mesh(),
    shape("rect", "fx-art__glass", { x: 40, y: TOP_Y, width: 240, height: BACK_Y - TOP_Y }),
    shape("polygon", "", { points: points(floor), fill: `url(#${id}-floor)` }),
    shape("path", "fx-art__lines", { d: `${svc(90)}${svc(142)}M160 90V142M40 ${BACK_Y}H280` }),
    shape("polygon", "fx-art__glass", { points: `40,${TOP_Y} 40,${BACK_Y} ${leftX(NEAR_Y)},${NEAR_Y} ${leftX(NEAR_Y)},${TOP_Y + 52}` }),
    shape("polygon", "fx-art__glass", { points: `280,${TOP_Y} 280,${BACK_Y} ${rightX(NEAR_Y)},${NEAR_Y} ${rightX(NEAR_Y)},${TOP_Y + 52}` }),
    ...glassPanels(),
  ]);
}

/** The net: mesh body, white tape on top, posts. Drawn between the two teams. */
export function net(): SVGElement {
  const l = leftX(NET_Y);
  const r = rightX(NET_Y);
  const top = NET_Y - NET_H;
  let d = "";
  for (let x = l; x <= r; x += 5) d += `M${x.toFixed(1)} ${top}V${NET_Y}`;
  for (let y = top + 3; y < NET_Y; y += 3) d += `M${l} ${y}H${r}`;
  return group("", {}, [
    shape("path", "fx-art__netshadow", { d: `M${l} ${NET_Y + 2}H${r}` }),
    shape("path", "fx-art__netmesh", { d }),
    shape("rect", "fx-art__tape", { x: l, y: top - 2, width: r - l, height: 3, rx: 1 }),
    shape("rect", "fx-art__post", { x: l - 3, y: top - 3, width: 4, height: NET_H + 4 }),
    shape("rect", "fx-art__post", { x: r - 1, y: top - 3, width: 4, height: NET_H + 4 }),
  ]);
}

/** Ball at `to` with a fading trail along the quadratic from `from` through `ctrl`. */
export function ball(from: Pt, ctrl: Pt, to: Pt): SVGElement {
  const kids: SVGElement[] = [];
  const steps = 9;
  for (let i = 0; i < steps; i++) {
    const t = 0.35 + (i / steps) * 0.6;
    const u = 1 - t;
    const x = u * u * from[0] + 2 * u * t * ctrl[0] + t * t * to[0];
    const y = u * u * from[1] + 2 * u * t * ctrl[1] + t * t * to[1];
    kids.push(shape("circle", "fx-art__trail", { cx: x.toFixed(1), cy: y.toFixed(1), r: (1.2 + i * 0.45).toFixed(2), "fill-opacity": ((i + 1) / (steps * 2.2)).toFixed(2) }));
  }
  kids.push(
    shape("circle", "fx-art__ballglow", { cx: to[0], cy: to[1], r: 10 }),
    shape("circle", "fx-art__ball", { cx: to[0], cy: to[1], r: 5.5 }),
    shape("path", "fx-art__ballseam", { d: `M${to[0] - 3.5} ${to[1] - 3.5}q3 3.5 0 7` }),
    shape("circle", "fx-art__ballgloss", { cx: to[0] + 1.8, cy: to[1] - 2, r: 1.5 }),
  );
  return group("", {}, kids);
}
