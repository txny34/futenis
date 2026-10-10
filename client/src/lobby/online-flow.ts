import { t } from "../i18n/t";
import { NetStatus, RemoteEngine } from "../net/remote-engine";
import { serverBase } from "../net/server-url";
import { accessToken, authUser } from "../auth/auth";
import { syncRatings } from "../auth/ratings";
import { rankOf } from "../profile/rank";
import { recorder } from "../profile/recorder";
import type { QueueMode } from "../ui/play-modal/play-modal";
import { arm, enterLobby, landingAllowed, type App } from "./flow";
import { cancelIntro } from "./intro";
import { endReplay } from "./replay-state";
import { isCurrent, leaveOnline, onlineOver, setRemote } from "./online-state";

// Online casual match from "Elegí un modo": the server pairs whoever is queued
// (cmd/server/queue.go) and the loop renders its frames instead of the local sim.
// Casual: no ELO, no profile stats, no replay (the tape needs the local sim).

function statusText(s: NetStatus): string {
  switch (s) {
    case "conectando": return t("Conectando…", "Connecting…");
    case "esperando rival": return t("Buscando jugadores…", "Finding players…");
    case "jugando": return "";
    case "el rival se fue": return t("El rival se fue", "Your opponent left");
    case "nadie se unió a la sala": return t("No apareció ningún rival. Probá de nuevo.", "No opponent showed up. Try again.");
    case "versión distinta, recargá": return t("Hay una versión nueva: recargá la página", "New version available: reload the page");
    case "desconectado por inactividad": return t("Desconectado por inactividad", "Disconnected due to inactivity");
    case "demasiados mensajes": return t("Desconectado: demasiados mensajes", "Disconnected: too many messages");
    case "iniciá sesión": return t("Las clasificatorias necesitan una cuenta: iniciá sesión", "Ranked needs an account: sign in");
    default: return t("Desconectado del servidor", "Disconnected from the server");
  }
}

const LIVE: ReadonlySet<NetStatus> = new Set<NetStatus>(["conectando", "esperando rival", "jugando"]);
const BACK_MS = 2500; // time to read why the session ended before the lobby
const NOTE_MS = 5000;

/** Status line while live; a session the server ended goes back to the lobby with its reason. */
function onStatus(app: App, remote: RemoteEngine, s: NetStatus, ping: number): void {
  if (!isCurrent(remote) || onlineOver()) return; // left already, or the result card is up
  const text = s === "jugando" ? `${ping} ms` : statusText(s);
  app.hud.error(text);
  if (LIVE.has(s)) return;
  window.setTimeout(() => {
    if (!isCurrent(remote) || onlineOver()) return;
    enterLobby(app);
    app.hud.error(text);
    window.setTimeout(() => app.hud.error(""), NOTE_MS);
  }, BACK_MS);
}

/** RESULT from the server: show the ELO change and refresh the profile from Supabase. */
function onRanked(app: App, mode: QueueMode, before: number, after: number): void {
  const d = after - before;
  const sign = d >= 0 ? "+" : "";
  app.hud.medal(`ELO ${mode}: ${before} → ${after} (${sign}${d}) · ${rankOf(after).label}`, performance.now());
  const u = authUser();
  if (u) void syncRatings(u.id);
}

let lastQueue: { mode: QueueMode; ranked: boolean } | null = null;

/** Revancha after an online match: back into the same queue (ranked stays ranked). */
export function requeue(app: App): void {
  if (lastQueue) startOnline(app, lastQueue.mode, lastQueue.ranked);
}

export function startOnline(app: App, mode: QueueMode, ranked = false): void {
  if (!app.loop) return;
  leaveOnline(app);
  endReplay(app);
  cancelIntro();
  recorder().end();
  const token = ranked ? accessToken() : null;
  if (ranked && !token) return; // the play modal only offers ranked when signed in
  lastQueue = { mode, ranked };
  const remote = new RemoteEngine(`${serverBase()}/ws?queue=${ranked ? `ranked${mode}` : mode}`, 0, token ?? "");
  remote.onResult = (before, after) => onRanked(app, mode, before, after);
  remote.onStatus = (s, ping) => onStatus(app, remote, s, ping);
  setRemote(remote);
  app.mode = mode;
  app.last = null; // Revancha requeues (see showResult); never a bot rematch
  app.state.holdUntil = 0;
  app.state.showLanding = landingAllowed(app.menu.settings, null);
  arm(app, false);
  app.loop.setEngine(remote);
  app.controls.lock();
  remote.connect();
}
