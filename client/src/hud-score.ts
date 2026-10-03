import { Mode, POINT_LABELS, Snap } from "./snapshot";

// Padel scoreboard (contact-v2 §21): 0/15/30/40/AD per team + games won +
// server dot, and a rally shot counter ("7 golpes") top-right that pulses on
// every new touch. DOM nodes are built once; text only changes on change.
const NAMES = ["Local", "Visita"] as const;
const UNSET = -1;
const PULSE_CLASS = "is-pulse";

function node<K extends keyof HTMLElementTagNameMap>(tag: K, cls: string, text = ""): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  e.className = cls;
  e.textContent = text;
  return e;
}

export class Scoreboard {
  readonly score = node("div", "");
  readonly shots = node("div", "");
  private readonly points = [node("b", "p0"), node("b", "p1")] as const;
  private readonly games = [node("span", "games g0"), node("span", "games g1")] as const;
  private readonly key = new Float64Array(7).fill(UNSET);
  private lastShots = UNSET;

  constructor() {
    this.score.id = "score";
    this.shots.id = "shots";
    this.shots.hidden = true;
    this.score.append(
      node("span", "team t0", NAMES[0]), this.games[0], this.points[0], node("i", "sep"),
      this.points[1], this.games[1], node("span", "team t1", NAMES[1]),
    );
  }

  update(snap: Float64Array): void {
    this.updateShots(snap);
    const k = this.key;
    const v0 = snap[Snap.Points0], v1 = snap[Snap.Points1], g0 = snap[Snap.Games0], g1 = snap[Snap.Games1];
    const server = snap[Snap.Server], mode = snap[Snap.Mode], sets = snap[Snap.Sets0] + snap[Snap.Sets1];
    if (k[0] === v0 && k[1] === v1 && k[2] === g0 && k[3] === g1 && k[4] === server && k[5] === mode && k[6] === sets) return;
    k[0] = v0; k[1] = v1; k[2] = g0; k[3] = g1; k[4] = server; k[5] = mode; k[6] = sets;
    this.score.hidden = mode === Mode.Playground;
    this.points[0].textContent = POINT_LABELS[v0] ?? String(v0);
    this.points[1].textContent = POINT_LABELS[v1] ?? String(v1);
    this.games[0].textContent = String(g0);
    this.games[1].textContent = String(g1);
    this.score.dataset.server = String(server);
  }

  private updateShots(snap: Float64Array): void {
    const n = snap[Snap.RallyShots];
    if (n === this.lastShots) return;
    const grew = n > this.lastShots && this.lastShots !== UNSET;
    this.lastShots = n;
    this.shots.hidden = n <= 0 || snap[Snap.Mode] === Mode.Playground;
    this.shots.textContent = n === 1 ? "1 golpe" : `${n} golpes`;
    if (!grew) return;
    this.shots.classList.remove(PULSE_CLASS);
    void this.shots.offsetWidth; // restart the CSS animation
    this.shots.classList.add(PULSE_CLASS);
  }
}
