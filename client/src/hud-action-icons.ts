// Action bar art: a little "toy athlete" doing each move, drawn as a sticker
// (thick dark outline under a white body) inside the 64x64 button, plus an
// effect layer (speed lines / ball trail) and the ball where it applies.
import { svgNode } from "./hud-svg";

export type IconId = "dash" | "strike" | "lob";

interface Figure {
  readonly body: string; // torso + limbs, stroked
  readonly head: readonly [number, number];
  readonly fx: string; // speed lines or ball trail
  readonly ball?: readonly [number, number];
}

const FIGURES: Readonly<Record<IconId, Figure>> = {
  dash: {
    body: "M36 25 L29 34 M34 27 L42 30 M34 27 L26 24 M29 34 L37 40 L39 48 M29 34 L23 41 L17 42",
    head: [38.5, 20],
    fx: "M12 25 H20 M10 31 H18 M13 37 H19",
  },
  strike: {
    body: "M32 25 L29 35 M31 27 L23 24 M31 27 L38 31 M29 35 L27 42 L25 49 M29 35 L38 37 L44 32",
    head: [33, 20],
    fx: "M35 44 Q41 42 44 37",
    ball: [48, 28],
  },
  lob: {
    body: "M25 30 L26 40 M25 32 L18 28 M25 32 L31 29 M26 40 L23 49 M26 40 L33 38 L36 32",
    head: [25, 25],
    fx: "M37 30 Q39 12 46 16",
    ball: [47, 19],
  },
};

const HEAD_R = 4.5;
const BALL_R = 3.6;

function dot(cls: string, at: readonly [number, number], r: number): SVGCircleElement {
  return svgNode("circle", { class: cls, cx: String(at[0]), cy: String(at[1]), r: String(r) });
}

/** The figure group for one action (fx, outline, body, head, ball). */
export function actionFigure(id: IconId): SVGGElement {
  const f = FIGURES[id];
  const g = svgNode("g", { class: "act__fig" });
  g.append(
    svgNode("path", { d: f.fx, class: "act__fx" }),
    svgNode("path", { d: f.body, class: "act__outline" }),
    dot("act__outline act__outline--dot", f.head, HEAD_R),
    svgNode("path", { d: f.body, class: "act__body" }),
    dot("act__head", f.head, HEAD_R),
  );
  if (f.ball) g.append(dot("act__ball", f.ball, BALL_R));
  return g;
}
