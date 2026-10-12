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
    const kind = emoteOf(e.code); // by position: AZERTY's unshifted digits are symbols
    if (kind < 0 || e.repeat || e.ctrlKey || e.metaKey || e.altKey) return;
    if (Object.values(app.menu.settings.controls.keys).includes(e.code)) return; // bound to a move: that wins
    if (!app.inLobby || app.menu.isOpen || app.play.isOpen || extraOverlayOpen()) return;
    if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return;
    const now = performance.now();
    if (now - last < EMOTE_EVERY_MS) return;
    last = now;
    app.view.players.emote(EMOTES[kind]);
    presence.emote(kind);
  });
}

/** Emote index for Digit1-4 / Numpad1-4, else -1. */
function emoteOf(code: string): number {
  const m = /^(?:Digit|Numpad)([1-9])$/.exec(code);
  const k = m ? Number(m[1]) - 1 : -1;
  return k < EMOTES.length ? k : -1;
}
