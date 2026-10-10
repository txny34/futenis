import type { RemoteEngine } from "../net/remote-engine";
import type { App } from "./flow";

// The live online match, if any. Kept apart from online-flow.ts so flow.ts can
// leave it (lobby, local match) without an import cycle (same as replay-state).
let remote: RemoteEngine | null = null;

export const isOnline = (): boolean => remote !== null;

export function setRemote(r: RemoteEngine): void { remote = r; }

/** Closes the socket and hands the loop back to the local wasm engine. Safe when offline. */
export function leaveOnline(app: App): void {
  if (!remote) return;
  remote.close();
  remote = null;
  app.loop?.setEngine(app.engine);
  app.hud.error("");
}
