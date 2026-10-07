import { athlete, PoseName } from "./figures";
import { ball, court, net, sceneDefs, stands } from "./scene";
import { group, Pt, svgRoot } from "./svg";

// Card art for "Elegí un modo": one composed scene per card so the four never look
// copy-pasted. Home team in front (green), rivals beyond the net (pink).
interface Placed { readonly x: number; readonly pose: PoseName; }
interface Spec {
  readonly id: string;
  readonly mirror: boolean;
  readonly home: readonly Placed[];
  readonly rival: readonly Placed[];
  readonly ball: readonly [Pt, Pt, Pt];
}

const SPECS: Readonly<Record<string, Spec>> = {
  "1v1-casual": { id: "pm-c1", mirror: false, home: [{ x: 128, pose: "kick" }], rival: [{ x: 196, pose: "ready" }],
    ball: [[160, 120], [175, 40], [192, 66]] },
  "2v2-casual": { id: "pm-c2", mirror: true, home: [{ x: 96, pose: "volley" }, { x: 214, pose: "ready" }],
    rival: [{ x: 128, pose: "ready" }, { x: 196, pose: "header" }], ball: [[110, 112], [150, 30], [192, 52]] },
  "1v1-ranked": { id: "pm-r1", mirror: true, home: [{ x: 150, pose: "header" }], rival: [{ x: 178, pose: "kick" }],
    ball: [[185, 92], [170, 40], [153, 58]] },
  "2v2-ranked": { id: "pm-r2", mirror: false, home: [{ x: 100, pose: "kick" }, { x: 208, pose: "sprint" }],
    rival: [{ x: 132, pose: "header" }, { x: 192, pose: "volley" }], ball: [[134, 118], [160, 34], [196, 62]] },
};

const RIVAL_FEET = 104;
const HOME_FEET = 152;

export function modeArt(mode: "1v1" | "2v2", ranked: boolean): SVGSVGElement {
  const s = SPECS[`${mode}-${ranked ? "ranked" : "casual"}`];
  const svg = svgRoot(`fx-art${ranked ? " fx-art--ranked" : ""}`, "0 0 320 160", true);
  if (s === undefined) return svg;
  const scene = group("", s.mirror ? { transform: "translate(320 0) scale(-1 1)" } : {}, [
    stands(s.id, ranked),
    court(s.id),
    ...s.rival.map((p) => athlete(p.x, RIVAL_FEET, 0.46, true, "rival", p.pose)),
    net(),
    ball(...s.ball),
    ...s.home.map((p) => athlete(p.x, HOME_FEET, 0.82, false, "home", p.pose)),
  ]);
  svg.append(sceneDefs(s.id, ranked), scene);
  return svg;
}
