import { EMOTES } from "../view/body-reaction";
import { extraOverlayOpen } from "../ui/overlays";
import type { App } from "./flow";
import type { Presence } from "./presence";

// Lobby emotes on the number keys: 1 saludo, 2 festejo, 3 baile, 4 bajón.
// My player plays it at once; the shared lobby relays it to everyone else
// (server throttled to one a second, so this mirrors that locally).

const EMOTE_EVERY_MS = 1000;

export function bindEmotes(app: App, presence: Presence): void {
  let last = -Infinity;
  window.addEventListener("keydown", (e) => {
    const kind = Number(e.key) - 1;
    if (!(kind >= 0 && kind < EMOTES.length) || e.repeat || e.ctrlKey || e.metaKey || e.altKey) return;
    if (!app.inLobby || app.menu.isOpen || app.play.isOpen || extraOverlayOpen()) return;
    if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return;
    const now = performance.now();
    if (now - last < EMOTE_EVERY_MS) return;
    last = now;
    app.view.players.emote(EMOTES[kind]);
    presence.emote(kind);
  });
}
