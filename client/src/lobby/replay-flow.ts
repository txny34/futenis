import type { MatchResult } from "../loop";
import { rankChange } from "../profile/rank";
import { recorder } from "../profile/recorder";
import { ReplayEngine } from "../replay/replay-engine";
import { Snap } from "../snapshot";
import { arm, configureMatch, enterLobby, errText, safeStart, type App } from "./flow";
import { startOnline } from "./online-flow";
import { isOnline } from "./online-state";

const HUMAN_TEAM = 0;

// "Ver repetición" (backlog "Repeticiones"): re-simulates the last match from
// its setup + tape on the same wasm engine. No profile/ELO side effects: the
// recorder's activity already closed with the real match.

export function canReplay(app: App): boolean {
  return app.last !== null && app.last.mode !== "playground" && app.tape.length > 0 && !app.tape.truncated;
}

export function startReplay(app: App): void {
  const o = app.last;
  if (!o || !canReplay(app) || !app.loop) return;
  // configure() alternates the first server per call: twice keeps the parity the
  // next real match (Revancha) expects; setConfig restores the original start.
  configureMatch(app, o);
  configureMatch(app, o);
  app.engine.setConfig(app.tape.startCfg);
  app.replaying = true;
  arm(app, false);
  app.controls.enabled = false; // the tape drives slot 0; the camera follows it
  app.state.holdUntil = 0;
  app.loop.setEngine(new ReplayEngine(app.engine, app.tape));
  app.hud.setReplay(true);
}


/** Match over (live or replay): profile/ELO once for the real match, then the result card. */
export function showResult(app: App, r: Readonly<MatchResult>, snap: Float64Array): void {
  app.controls.enabled = false;
  app.controls.releaseAll();
  const winner = r.winner >= 0 ? r.winner : snap[Snap.Sets0] >= snap[Snap.Sets1] ? 0 : 1;
  const elo = app.replaying ? null : recorder().matchOver(winner === HUMAN_TEAM, r.games0, r.games1);
  const rankNote = elo ? rankChange(elo.before, elo.after) : null;
  app.screens.showResult({ won: winner === HUMAN_TEAM, games0: r.games0, games1: r.games1, rankNote }, {
    rematch: () => {
      if (isOnline() && (app.mode === "1v1" || app.mode === "2v2")) startOnline(app, app.mode); // back in the queue
      else if (app.last) safeStart(app, app.last);
    },
    lobby: () => {
      try { enterLobby(app); } catch (err) { app.hud.error(errText(err)); }
    },
    replay: canReplay(app) ? () => {
      try { startReplay(app); } catch (err) { app.hud.error(errText(err)); }
    } : undefined,
  });
}
