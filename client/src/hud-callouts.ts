import { HypeMoment, LONG_RALLY, STREAK_CALL } from "./feel/hype";
import { ShotKind, ShotKindId } from "./feel/shots";
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
  [ShotKind.Chilena]: { text: "¡CHILENA!", prio: 50, tone: "gold" },
  [ShotKind.Cabezazo]: { text: "¡CABEZAZO!", prio: 45, tone: "cyan" },
  [ShotKind.Remate]: { text: "¡REMATE!", prio: 45, tone: "hot" },
  [ShotKind.Volea]: { text: "¡VOLEA!", prio: 30, tone: "cyan" },
  [ShotKind.Globo]: { text: "¡GLOBO!", prio: 30, tone: "cyan" },
};

const EVENT_CALLS: Readonly<Record<number, Callout>> = {
  [EventKind.SetWon]: { text: "¡SET!", prio: 90, tone: "gold" },
  [EventKind.Ace]: { text: "¡ACE!", prio: 85, tone: "gold" },
  [EventKind.GameWon]: { text: "¡JUEGO!", prio: 80, tone: "gold" },
  [EventKind.BreakPoint]: { text: "BOLA DE RUPTURA", prio: 60, tone: "hot" },
  [EventKind.GamePoint]: { text: "PUNTO DE JUEGO", prio: 60, tone: "cyan" },
  [EventKind.HitNet]: { text: "¡RED!", prio: 40, tone: "hot" },
};

const PUNTAZO: Callout = { text: "¡PUNTAZO!", prio: 55, tone: "gold" };
const STREAK_TONE = "hot";
const STREAK_PRIO = 52; // moments sit under PUNTO DE JUEGO / BOLA DE RUPTURA (60): score info wins

const RIVAL_POINT: Callout = { text: "PUNTO RIVAL", prio: 20, tone: "hot" };

export class Callouts {
  readonly root = document.createElement("div");
  private prio = 0;
  private until = 0;
  private outAt = 0;

  constructor() {
    this.root.id = "banner";
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
      this.show({ text: `¡RACHA DE ${m.streak}!`, prio: STREAK_PRIO, tone: STREAK_TONE }, nowMs);
    }
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
