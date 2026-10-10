import { applyThemeCss } from "./env/theme";
import { Hud } from "./hud";
import { LANG, t } from "./i18n/t";
import { Controls } from "./input";
import { GameLoop, LoopState } from "./loop";
import { NetStatus, RemoteEngine } from "./net/remote-engine";
import { CourtView } from "./render";

const SERVER_PORT = 8091;
declare const __FUTENIS_SERVER__: string; // injected by cmd/dev's esbuild Define (FUTENIS_SERVER)
const CODE_LEN = 5;
const CODE_CHARS = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789";
const MAX_LAG_MS = 1000;
const ROOM_RE = /^[A-Z0-9]{3,12}$/;

function serverBase(): string {
  if (__FUTENIS_SERVER__) return __FUTENIS_SERVER__;
  return `${location.protocol === "https:" ? "wss" : "ws"}://${location.hostname}:${SERVER_PORT}`;
}

function requireElement(id: string): HTMLElement {
  const e = document.getElementById(id);
  if (!e) throw new Error(`${t("Falta el elemento", "Missing element")} #${id} ${t("en", "in")} online.html`);
  return e;
}

/** Player-visible text for each NetStatus value (the union itself stays in Spanish: it's
 * compared in code as an internal identifier, e.g. against "jugando"/CLOSE_STATUS). */
function statusText(s: NetStatus): string {
  switch (s) {
    case "conectando": return t("conectando", "connecting");
    case "esperando rival": return t("esperando rival", "waiting for opponent");
    case "jugando": return t("jugando", "playing");
    case "el rival se fue": return t("el rival se fue", "opponent left");
    case "desconectado": return t("desconectado", "disconnected");
    case "versión distinta, recargá": return t("versión distinta, recargá", "different version, reload");
    case "desconectado por inactividad": return t("desconectado por inactividad", "disconnected due to inactivity");
    case "nadie se unió a la sala": return t("nadie se unió a la sala", "nobody joined the room");
    case "demasiados mensajes": return t("demasiados mensajes", "too many messages");
    default: return s;
  }
}

function randomCode(): string {
  let s = "";
  const bytes = crypto.getRandomValues(new Uint8Array(CODE_LEN));
  for (const b of bytes) s += CODE_CHARS[b % CODE_CHARS.length];
  return s;
}

function roomFromUrl(params: URLSearchParams): string {
  const room = (params.get("room") ?? "").toUpperCase();
  if (ROOM_RE.test(room)) return room;
  const code = randomCode();
  params.set("room", code);
  history.replaceState(null, "", `?${params.toString()}`);
  return code;
}

function boot(): void {
  document.documentElement.lang = LANG;
  applyThemeCss(document.documentElement); // #net lives outside #hud: needs the --hud-* tokens too
  const params = new URLSearchParams(location.search);
  const room = roomFromUrl(params);
  const lag = Math.max(0, Math.min(MAX_LAG_MS, Number(params.get("lag")) || 0));
  const hud = new Hud(requireElement("hud"));
  const view = new CourtView(requireElement("app"));
  const net = requireElement("net");
  const controls = new Controls();
  controls.attach(view.canvas);
  controls.onLook = (dx, dy) => view.rig.look(dx, dy);
  controls.onZoom = (d) => view.rig.zoom(d);
  controls.onLockChange = (locked) => hud.setCrosshair(locked);
  const state: LoopState = { running: true, paused: false, fpsCap: 0, cameraShake: true, holdUntil: 0, showLanding: true };
  const engine = new RemoteEngine(`${serverBase()}/ws?room=${room}`, lag);
  const lagText = lag > 0 ? ` · ${t("lag simulado", "simulated lag")} ${lag} ms` : "";
  engine.onStatus = (s, ping) => {
    net.textContent = `${t("Sala", "Room")} ${room} · ${statusText(s)}${s === "jugando" ? ` · ${ping} ms` : ""}${lagText}`;
  };
  view.setLobby(false);
  view.rig.face(0);
  controls.enabled = true;
  hud.resetMatch();
  new GameLoop(engine, view, hud, controls, state).start();
  engine.connect();
}

try {
  boot();
} catch (err: unknown) {
  const root = document.getElementById("hud");
  if (root) root.textContent = `${t("Error", "Error")}: ${err instanceof Error ? err.message : String(err)}`;
}
