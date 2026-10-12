import type { RemoteEngine } from "../net/remote-engine";
import type { App } from "./flow";

// The live online match, if any. Kept apart from online-flow.ts so flow.ts can
// leave it (lobby, local match) without an import cycle (same as replay-state).
let remote: RemoteEngine | null = null;
let over = false;

export const isOnline = (): boolean => remote !== null;
const leaveCbs: Array<() => void> = [];
/** Runs whenever the online session ends (lobby, local match, new session). */
export function onLeaveOnline(cb: () => void): void { leaveCbs.push(cb); }
/** True once the online match reached its result screen (later disconnects are moot). */
export const onlineOver = (): boolean => over;
export function markOnlineOver(): void { over = true; }
export const isCurrent = (r: RemoteEngine): boolean => remote === r;

export function setRemote(r: RemoteEngine): void { remote = r; over = false; }

/** Closes the socket and hands the loop back to the local wasm engine. Safe when offline. */
export function leaveOnline(app: App): void {
  if (!remote) return;
  remote.close();
  remote = null;
  app.loop?.setEngine(app.engine);
  app.view.players.setLooks(null); // back to the bots' looks
  app.hud.error("");
  for (const cb of leaveCbs) cb();
}
