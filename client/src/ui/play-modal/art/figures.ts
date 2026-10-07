import { group, Pt, shape } from "./svg";

// Stylized foot-padel athletes (no rackets): shaded torso, head with hair, two-tone
// limbs and shoes. Local units: hip at (0,0), feet ~ +38, head top ~ -62, facing +x.
export interface Pose {
  readonly backKnee: Pt; readonly backFoot: Pt;
  readonly frontKnee: Pt; readonly frontFoot: Pt;
  readonly backElbow: Pt; readonly backHand: Pt;
  readonly frontElbow: Pt; readonly frontHand: Pt;
  readonly lean: number;
}

export const POSES = {
  kick: { backKnee: [-5, 19], backFoot: [-8, 38], frontKnee: [16, 4], frontFoot: [36, -8],
    backElbow: [-18, -28], backHand: [-28, -16], frontElbow: [14, -30], frontHand: [24, -40], lean: -7 },
  ready: { backKnee: [-11, 18], backFoot: [-13, 38], frontKnee: [11, 18], frontFoot: [13, 38],
    backElbow: [-17, -24], backHand: [-15, -10], frontElbow: [17, -24], frontHand: [15, -10], lean: 2 },
  volley: { backKnee: [-3, 20], backFoot: [-5, 38], frontKnee: [15, 8], frontFoot: [12, 26],
    backElbow: [-20, -34], backHand: [-30, -44], frontElbow: [18, -26], frontHand: [29, -18], lean: 5 },
  header: { backKnee: [-9, 13], backFoot: [-5, 29], frontKnee: [9, 13], frontFoot: [15, 27],
    backElbow: [-19, -42], backHand: [-23, -56], frontElbow: [19, -42], frontHand: [23, -56], lean: 0 },
  sprint: { backKnee: [-12, 14], backFoot: [-24, 26], frontKnee: [12, 12], frontFoot: [10, 34],
    backElbow: [-14, -22], backHand: [-6, -12], frontElbow: [12, -38], frontHand: [22, -30], lean: 9 },
} as const satisfies Record<string, Pose>;
export type PoseName = keyof typeof POSES;

const HIP_W = 7;
const SHOULDER_Y = -38;

function limb(a: Pt, b: Pt, c: Pt, upper: string, lower: string, w: number): SVGElement[] {
  return [
    shape("path", `fx-fig__limb ${lower}`, { d: `M${b[0]} ${b[1]}L${c[0]} ${c[1]}`, "stroke-width": w - 1 }),
    shape("path", `fx-fig__limb ${upper}`, { d: `M${a[0]} ${a[1]}L${b[0]} ${b[1]}`, "stroke-width": w }),
  ];
}

function leg(knee: Pt, foot: Pt, hipX: number): SVGElement[] {
  return [
    ...limb([hipX, 0], knee, foot, "fx-fig__shorts", "fx-fig__sock", 8),
    shape("ellipse", "fx-fig__shoe", { cx: foot[0] + 2.5, cy: foot[1] + 1, rx: 5.5, ry: 3 }),
  ];
}

function arm(elbow: Pt, hand: Pt, sx: number): SVGElement[] {
  return [
    ...limb([sx, SHOULDER_Y + 3], elbow, hand, "fx-fig__sleeve", "fx-fig__skin", 6),
    shape("circle", "fx-fig__hand", { cx: hand[0], cy: hand[1], r: 2.8 }),
  ];
}

function torso(lean: number): SVGElement[] {
  const s = SHOULDER_Y;
  return [
    shape("path", "fx-fig__jersey", { d: `M${-HIP_W} 2L${-11 + lean} ${s}H${11 + lean}L${HIP_W} 2Z` }),
    shape("path", "fx-fig__shade", { d: `M${lean * 0.5} ${s}H${11 + lean}L${HIP_W} 2H0Z` }),
    shape("path", "fx-fig__stripe", { d: `M${-9 + lean * 0.9} ${s + 9}H${9 + lean * 0.9}` }),
    shape("path", "fx-fig__band", { d: `M${-HIP_W - 1} 1H${HIP_W + 1}` }),
  ];
}

function head(lean: number): SVGElement[] {
  const x = lean * 1.2;
  return [
    shape("path", "fx-fig__limb fx-fig__skin", { d: `M${lean} ${SHOULDER_Y}L${x} ${SHOULDER_Y - 6}`, "stroke-width": 5 }),
    shape("circle", "fx-fig__face", { cx: x, cy: -52, r: 9.5 }),
    shape("path", "fx-fig__hair", { d: `M${x - 9.5} -53a9.5 9.5 0 0 1 19 -1c-5-3-12-3-19 1z` }),
    shape("circle", "fx-fig__gloss", { cx: x + 4, cy: -54, r: 2.4 }),
  ];
}

/** team: "home" | "rival" (jersey palette); flip faces -x. */
export function athlete(x: number, y: number, scale: number, flip: boolean, team: "home" | "rival", pose: PoseName): SVGElement {
  const p = POSES[pose];
  const shadow = shape("ellipse", "fx-fig__shadow", { cx: x, cy: y + 2 * scale, rx: 16 * scale, ry: 4 * scale });
  const body = group(`fx-fig fx-fig--${team}`, { transform: `translate(${x} ${y - 38 * scale}) scale(${flip ? -scale : scale} ${scale})` }, [
    ...leg(p.backKnee, p.backFoot, -3),
    ...arm(p.backElbow, p.backHand, -9 + p.lean),
    ...torso(p.lean),
    ...head(p.lean),
    ...leg(p.frontKnee, p.frontFoot, 3),
    ...arm(p.frontElbow, p.frontHand, 9 + p.lean),
  ]);
  return group("", {}, [shadow, body]);
}
