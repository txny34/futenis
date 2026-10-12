import { t } from "./i18n/t";
import { Mode, Snap } from "./snapshot";

// Scoreboard: sets won + points of the current set per team (best of 3 sets,
// each first to 5, win by 2) + server dot, a SET/MATCH POINT glow on the
// team one point away (sim EvSetPoint / EvMatchPoint; every EvPoint
// clears it first, Hud.onEvent; after a mid-match rejoin it shows from the next point), and a rally shot counter ("7 golpes") top-right that pulses on
// every new touch. DOM nodes are built once; text only changes on change.
const NAMES = [t("Local", "Home"), t("Visita", "Away")] as const;
const UNSET = -1;
const PULSE_CLASS = "is-pulse";
const MP_CLASS = "is-mp";
const SP_CLASS = "is-sp";

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
  private readonly sets = [node("span", "sets s0"), node("span", "sets s1")] as const;
  private readonly key = new Float64Array(6).fill(UNSET);
  private lastShots = UNSET;

  constructor() {
    this.score.id = "score";
    this.shots.id = "shots";
    this.shots.hidden = true;
    this.score.append(
      node("span", "team t0", NAMES[0]), this.sets[0], this.points[0], node("i", "sep"),
      this.points[1], this.sets[1], node("span", "team t1", NAMES[1]),
    );
  }

  update(snap: Float64Array): void {
    this.updateShots(snap);
    const k = this.key;
    const score0 = snap[Snap.Score0], score1 = snap[Snap.Score1], server = snap[Snap.Server], mode = snap[Snap.Mode];
    const sets0 = snap[Snap.Sets0], sets1 = snap[Snap.Sets1];
    if (k[0] === score0 && k[1] === score1 && k[2] === server && k[3] === mode && k[4] === sets0 && k[5] === sets1) return;
    if (score0 + score1 === 0) this.matchPoint(-1, false); // a fresh set or match
    k[0] = score0; k[1] = score1; k[2] = server; k[3] = mode; k[4] = sets0; k[5] = sets1;
    this.score.hidden = mode === Mode.Playground;
    this.points[0].textContent = String(score0);
    this.points[1].textContent = String(score1);
    this.sets[0].textContent = String(sets0);
    this.sets[1].textContent = String(sets1);
    const match = mode === Mode.OneVsOne || mode === Mode.TwoVsTwo; // the lobby is a single tally
    this.sets[0].hidden = !match;
    this.sets[1].hidden = !match;
    this.score.dataset.server = String(server);
  }

  /** EvMatchPoint / EvSetPoint for side (or -1 to clear): glows that team's points. */
  matchPoint(side: number, match: boolean): void {
    if (side < 0) {
      for (const p of this.points) p.classList.remove(MP_CLASS, SP_CLASS);
      return;
    }
    this.points[side === 1 ? 1 : 0].classList.add(match ? MP_CLASS : SP_CLASS);
  }

  private updateShots(snap: Float64Array): void {
    const n = snap[Snap.RallyShots];
    if (n === this.lastShots) return;
    const grew = n > this.lastShots && this.lastShots !== UNSET;
    this.lastShots = n;
    this.shots.hidden = n <= 0 || snap[Snap.Mode] === Mode.Playground;
    this.shots.textContent = n === 1 ? t("1 golpe", "1 touch") : t(`${n} golpes`, `${n} touches`);
    if (!grew) return;
    this.shots.classList.remove(PULSE_CLASS);
    void this.shots.offsetWidth; // restart the CSS animation
    this.shots.classList.add(PULSE_CLASS);
  }
}
