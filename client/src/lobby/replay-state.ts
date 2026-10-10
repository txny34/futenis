import type { App } from "./flow";

/** Back to the live engine (lobby, new match). Safe to call when not replaying. */
export function endReplay(app: App): void {
  if (!app.replaying) return;
  app.replaying = false;
  app.loop?.setEngine(app.engine);
  app.hud.setReplay(false);
}
