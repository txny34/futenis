import { applyThemeCss } from "./env/theme";
import { Hud } from "./hud";
import { Controls } from "./input";
import { GameLoop, LoopState } from "./loop";
import { RemoteEngine } from "./net/remote-engine";
import { CourtView } from "./render";

const SERVER_PORT = 8091;
const CODE_LEN = 5;
const CODE_CHARS = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789";
const MAX_LAG_MS = 1000;
const ROOM_RE = /^[A-Z0-9]{3,12}$/;

function requireElement(id: string): HTMLElement {
  const e = document.getElementById(id);
  if (!e) throw new Error(`Falta el elemento #${id} en online.html`);
  return e;
}

function randomCode(): string {
  let s = "";
  for (let i = 0; i < CODE_LEN; i++) s += CODE_CHARS[Math.floor(Math.random() * CODE_CHARS.length)];
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
  const engine = new RemoteEngine(`ws://${location.hostname}:${SERVER_PORT}/ws?room=${room}`, lag);
  const lagText = lag > 0 ? ` · lag simulado ${lag} ms` : "";
  engine.onStatus = (s, ping) => {
    net.textContent = `Sala ${room} · ${s}${s === "jugando" ? ` · ${ping} ms` : ""}${lagText}`;
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
  if (root) root.textContent = `Error: ${err instanceof Error ? err.message : String(err)}`;
}
