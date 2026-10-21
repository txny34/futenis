import type { SimEvent } from "./snapshot";
import type { GameLoop } from "./loop";
import { LONG_RALLY } from "./feel/hype";
import { t } from "./i18n/t";
import { MEDALS } from "./profile/challenges";
import { recorder } from "./profile/recorder";
import { App, onPadEvent } from "./lobby/flow";
import { showResult } from "./lobby/replay-flow";
import { isOnline } from "./lobby/online-state";
import { recordStep } from "./lobby/replay-save";

// What the game loop reports goes here: results, the replay tape, events to
// sound/tutorial/medals, crowd hype, lobby pads and per-frame audio (steps).

const HYPE_STREAK_ROAR = 0.6; // a streak gets a smaller roar than a puntazo
const MEDAL_ROAR = 0.5;

export function hookLoop(app: App, loop: GameLoop): void {
  loop.onMatchOver = (r, snap) => showResult(app, r, snap);
  loop.onStep = (n, a) => { if (!app.replaying && !isOnline() && app.tape) recordStep(app.tape, n, a); };
  loop.onGameEvent = (e) => { recorder().onEvent(e); app.sound.onEvent(e); app.tutorial.onEvent(e); onMedal(app, e); };
  loop.onHype = (m) => app.sound.cheer(m.touches >= LONG_RALLY ? 1 : HYPE_STREAK_ROAR);
  loop.onPadEvent = (kind, pad) => onPadEvent(app, kind, pad);
  loop.onFrame = (snap, dt) => { app.sound.frame(snap, dt); app.replay.frame(); };
}

function onMedal(app: App, e: SimEvent): void {
  const earned = app.challenges.onEvent(e);
  if (earned.length === 0) return;
  const top = earned[earned.length - 1]; // banner shows one; every medal is saved
  app.hud.medal(t(`¡${MEDALS[top.medal].toUpperCase()}! ${top.def.title}`, `${MEDALS[top.medal].toUpperCase()}! ${top.def.title}`), performance.now());
  app.sound.cheer(MEDAL_ROAR);
}
