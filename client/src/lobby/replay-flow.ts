import type { MatchResult } from "../loop";
import { rankChange } from "../profile/rank";
import { recorder } from "../profile/recorder";
import { t } from "../i18n/t";
import { loadReplay } from "../replay/store";
import { Snap } from "../snapshot";
import { enterLobby, errText, safeStart, type App } from "./flow";
import { requeue } from "./online-flow";
import { isOnline, markOnlineOver } from "./online-state";
import { noteResult } from "./replay-save";

const HUMAN_TEAM = 0;

// "Ver repetición" (backlog "Repeticiones"): plays the last match's tape v2 (or one
// stored in Historial) through lobby/replay-player.ts. No profile/ELO side effects.

export function canReplay(app: App): boolean {
  return app.lastReplay !== null;
}

export function startReplay(app: App): void {
  if (!app.lastReplay) return;
  try { app.replay.play(app.lastReplay.tape, app.lastReplay.meta); } catch (err) { app.hud.error(errText(err)); }
}

/** Historial ▶: a replay saved in this browser. */
let openRequest = 0;

/** Historial ▶. Dropped if the player started something else while it loaded. */
export async function openStoredReplay(app: App, id: number): Promise<void> {
  const mine = ++openRequest;
  const r = await loadReplay(id);
  if (mine !== openRequest || !app.inLobby || isOnline()) return;
  if (!r) { app.hud.error(t("No se encontró la repetición.", "Replay not found.")); return; }
  try { app.replay.play(r.tape, r.meta); } catch {
    app.hud.error(t("No se pudo abrir la repetición (versión vieja del juego).", "Couldn't open the replay (older game version)."));
  }
}

/** Profile/ELO + history record, and the replay saved under the record's `at`. */
function record(app: App, won: boolean, p0: number, p1: number): { before: number; after: number } | null {
  const at = Date.now();
  const elo = recorder().matchOver(won, p0, p1, at);
  const queue = app.mode;
  if (queue !== "1v1" && queue !== "2v2") return elo;
  const online = isOnline();
  const ranked = online ? app.onlineRanked : app.last?.ranked ?? false;
  noteResult(app, { id: at, queue, online, ranked, won, games0: p0, games1: p1, looks: online ? app.onlineLooks : null });
  return elo;
}

/** Match over (live or replay): profile/ELO once for the real match, then the result card. */
export function showResult(app: App, r: Readonly<MatchResult>, snap: Float64Array): void {
  if (app.replaying) { app.replay.onEnded(); return; }
  app.controls.enabled = false;
  app.controls.releaseAll();
  if (isOnline()) markOnlineOver();
  const winner = r.winner >= 0 ? r.winner : snap[Snap.Sets0] >= snap[Snap.Sets1] ? 0 : 1;
  // The final sets come from the snapshot (exact after a rejoin or dropped events).
  const p0 = snap[Snap.Sets0], p1 = snap[Snap.Sets1];
  const elo = record(app, winner === HUMAN_TEAM, p0, p1);
  const rankNote = elo ? rankChange(elo.before, elo.after) : null;
  app.screens.showResult({ won: winner === HUMAN_TEAM, games0: p0, games1: p1, rankNote }, {
    rematch: () => {
      if (isOnline()) requeue(app); // back in the same queue
      else if (app.last) safeStart(app, app.last);
    },
    lobby: () => {
      try { enterLobby(app); } catch (err) { app.hud.error(errText(err)); }
    },
    replay: canReplay(app) ? () => startReplay(app) : undefined,
  });
}
