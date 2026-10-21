import type { Looks } from "../net/style-wire";
import type { ReplayMeta } from "../replay/store";
import type { TapeWriter } from "../replay/tape-format";
import type { App } from "./flow";
import type { ReplayPlayer } from "./replay-player";

/** Replay fields of App (kept here so flow.ts stays under its size cap). */
export interface ReplayAppFields {
  /** Tape of the local bot match being recorded; null for online/playground/lobby. */
  tape: TapeWriter | null;
  /** The last finished match's replay ("Ver repetición" on the result card). */
  lastReplay: { meta: ReplayMeta; tape: Uint8Array } | null;
  /** Online: latest seat looks (STYLES) and whether the queue is ranked, for the replay meta. */
  onlineLooks: Looks | null;
  onlineRanked: boolean;
  /** Plays replays with the mini HUD. */
  replay: ReplayPlayer;
  replaying: boolean;
}

/** Back to the live engine (lobby, new match). Safe to call when not replaying. */
export function endReplay(app: App): void {
  if (!app.replaying) return;
  app.replaying = false;
  app.replay.stop();
  app.loop?.setEngine(app.engine);
  app.hud.setReplay(false);
}
