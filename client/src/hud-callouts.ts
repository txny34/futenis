import { HypeMoment, LONG_RALLY, STREAK_CALL } from "./feel/hype";
import { ShotKind, ShotKindId } from "./feel/shots";
import { t } from "./i18n/t";
import { EventKind, SimEvent } from "./snapshot";

// Big centered banner (top third) for shot names and padel moments. A new
// callout replaces the current one only if it is at least as important or
// the current one already expired. Animated in/out with CSS classes.
const BANNER_MS = 1300;
const OUT_MS = 250;
const IN_CLASS = "is-in";
const OUT_CLASS = "is-out";

interface Callout { readonly text: string; readonly prio: number; readonly tone: string }

const SHOT_CALLS: Readonly<Record<number, Callout>> = {
  [ShotKind.Chilena]: { text: t("¡CHILENA!", "BICYCLE KICK!"), prio: 50, tone: "gold" },
  [ShotKind.Cabezazo]: { text: t("¡CABEZAZO!", "HEADER!"), prio: 45, tone: "cyan" },
  [ShotKind.Remate]: { text: t("¡REMATE!", "SMASH!"), prio: 45, tone: "hot" },
  [ShotKind.Volea]: { text: t("¡VOLEA!", "VOLLEY!"), prio: 30, tone: "cyan" },
  [ShotKind.Globo]: { text: t("¡GLOBO!", "LOB!"), prio: 30, tone: "cyan" },
};

const EVENT_CALLS: Readonly<Record<number, Callout>> = {
  [EventKind.SetWon]: { text: t("¡SET!", "SET!"), prio: 90, tone: "gold" },
  [EventKind.MatchPoint]: { text: t("MATCH POINT", "MATCH POINT"), prio: 70, tone: "hot" },
  [EventKind.SetPoint]: { text: t("SET POINT", "SET POINT"), prio: 65, tone: "cyan" },
  [EventKind.Ace]: { text: t("¡ACE!", "ACE!"), prio: 85, tone: "gold" },
  [EventKind.GameWon]: { text: t("¡JUEGO!", "GAME!"), prio: 80, tone: "gold" },
  [EventKind.BreakPoint]: { text: t("BOLA DE RUPTURA", "BREAK POINT"), prio: 60, tone: "hot" },
  [EventKind.GamePoint]: { text: t("PUNTO DE JUEGO", "GAME POINT"), prio: 60, tone: "cyan" },
  [EventKind.HitNet]: { text: t("¡RED!", "NET!"), prio: 40, tone: "hot" },
};

const PUNTAZO: Callout = { text: t("¡PUNTAZO!", "WHAT A POINT!"), prio: 55, tone: "gold" };
const STREAK_TONE = "hot";
const STREAK_PRIO = 52; // moments sit under PUNTO DE JUEGO / BOLA DE RUPTURA (60): score info wins

const RIVAL_POINT: Callout = { text: t("PUNTO RIVAL", "OPPONENT'S POINT"), prio: 20, tone: "hot" };

export class Callouts {
  readonly root = document.createElement("div");
  private prio = 0;
  private until = 0;
  private outAt = 0;

  constructor() {
    this.root.id = "banner";
    this.root.setAttribute("role", "status");
    this.root.setAttribute("aria-live", "polite");
    this.root.hidden = true;
  }

  onEvent(e: SimEvent, shot: ShotKindId, humanTeam: number, nowMs: number): void {
    const c = shot !== ShotKind.None ? SHOT_CALLS[shot]
      : e.kind === EventKind.Point && e.side !== humanTeam ? RIVAL_POINT
        : EVENT_CALLS[e.kind];
    if (c) this.show(c, nowMs);
  }

  /** Long rally (either team: the crowd loves it) or the human team's streak. */
  onMoment(m: Readonly<HypeMoment>, humanTeam: number, nowMs: number): void {
    if (m.touches >= LONG_RALLY) this.show(PUNTAZO, nowMs);
    else if (m.winner === humanTeam && m.streak >= STREAK_CALL) {
      const text = t(`¡RACHA DE ${m.streak}!`, `STREAK OF ${m.streak}!`);
      this.show({ text, prio: STREAK_PRIO, tone: STREAK_TONE }, nowMs);
    }
  }

  /** Free-form banner (training medals). */
  announce(text: string, prio: number, tone: string, nowMs: number): void {
    this.show({ text, prio, tone }, nowMs);
  }

  update(nowMs: number): void {
    if (this.root.hidden) return;
    if (nowMs > this.until) {
      this.root.hidden = true;
      this.prio = 0;
    } else if (nowMs > this.outAt && !this.root.classList.contains(OUT_CLASS)) {
      this.root.classList.add(OUT_CLASS);
    }
  }

  clear(): void {
    this.root.hidden = true;
    this.prio = 0;
  }

  private show(c: Callout, nowMs: number): void {
    if (!this.root.hidden && c.prio < this.prio && nowMs <= this.until) return;
    this.prio = c.prio;
    this.until = nowMs + BANNER_MS;
    this.outAt = this.until - OUT_MS;
    this.root.textContent = c.text;
    this.root.dataset.tone = c.tone;
    this.root.classList.remove(IN_CLASS, OUT_CLASS);
    this.root.hidden = false;
    void this.root.offsetWidth; // restart the CSS animation
    this.root.classList.add(IN_CLASS);
  }
}
