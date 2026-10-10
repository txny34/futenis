import { t } from "../i18n/t";
import { NetStatus, RemoteEngine } from "../net/remote-engine";
import { parseRoomCode, randomRoomCode, roomLink } from "../net/room-code";
import { serverBase } from "../net/server-url";
import { SearchPanel } from "../ui/search-panel";
import { accessToken, authUser } from "../auth/auth";
import { syncRatings } from "../auth/ratings";
import { rankOf } from "../profile/rank";
import { recorder } from "../profile/recorder";
import type { QueueMode } from "../ui/play-modal/play-modal";
import { arm, enterLobby, landingAllowed, type App } from "./flow";
import { cancelIntro } from "./intro";
import { endReplay } from "./replay-state";
import { isCurrent, leaveOnline, onLeaveOnline, onlineOver, setRemote } from "./online-state";

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
const JOIN_POLL_MS = 200;

let panel: SearchPanel | null = null;
let shareLink = ""; // private room: the link shown while waiting

function searchPanel(app: App): SearchPanel {
  if (!panel) {
    panel = new SearchPanel(document.body, () => enterLobby(app));
    onLeaveOnline(() => panel?.hide()); // lobby, local match or a new session
  }
  return panel;
}

/** Search panel while waiting, ping while playing; a session the server ended
 * goes back to the lobby with its reason. */
function onStatus(app: App, remote: RemoteEngine, s: NetStatus, ping: number): void {
  if (!isCurrent(remote) || onlineOver()) return; // left already, or the result card is up
  const waiting = s === "conectando" || s === "esperando rival";
  if (waiting) searchPanel(app).show(statusText(s), shareLink);
  else panel?.hide();
  const text = s === "jugando" ? `${ping} ms` : waiting ? "" : statusText(s);
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
  const token = ranked ? accessToken() : null;
  if (ranked && !token) return; // the play modal only offers ranked when signed in
  lastQueue = { mode, ranked };
  shareLink = "";
  connect(app, mode, `${serverBase()}/ws?queue=${ranked ? `ranked${mode}` : mode}`, token ?? "");
}

/** "Jugar con un amigo": a private 1v1 room; the friend opens the link. */
export function startFriendRoom(app: App, code = randomRoomCode()): void {
  lastQueue = null; // Revancha after a friend match: back to the lobby menu
  shareLink = roomLink(code);
  connect(app, "1v1", `${serverBase()}/ws?room=${code}`, "");
}

/** Boot with ?room=CODE (a friend's link): join that room once the game is up. */
export function joinFromUrl(app: App): void {
  const code = parseRoomCode(new URLSearchParams(location.search).get("room"));
  if (!code) return;
  history.replaceState(null, "", location.pathname); // a reload must not rejoin
  const tryJoin = (): void => {
    if (app.engine.loaded && app.inLobby) startFriendRoom(app, code);
    else window.setTimeout(tryJoin, JOIN_POLL_MS);
  };
  tryJoin();
}

function connect(app: App, mode: QueueMode, url: string, token: string): void {
  if (!app.loop) return;
  leaveOnline(app);
  endReplay(app);
  cancelIntro();
  recorder().end();
  const remote = new RemoteEngine(url, 0, token);
  remote.onResult = (before, after) => onRanked(app, mode, before, after);
  remote.onStatus = (s, ping) => onStatus(app, remote, s, ping);
  setRemote(remote);
  app.mode = mode;
  app.last = null; // Revancha requeues (see showResult); never a bot rematch
  app.state.holdUntil = 0;
  app.state.showLanding = landingAllowed(app.menu.settings, null);
  arm(app, false);
  app.loop.setEngine(remote);
  app.controls.releaseAll(); // pointer stays free for the search panel; a click on the court grabs it
  remote.connect();
}
