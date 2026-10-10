import { t } from "../i18n/t";
import { LobbyMsg, MAX_NAME_BYTES, PRESENCE_LEN, ROSTER_STRIDE } from "../net/lobby-proto";
import { serverBase } from "../net/server-url";
import { displayName } from "../profile/display-name";
import { profileStore } from "../profile/store";
import { Snap } from "../snapshot";
import { isOnline } from "./online-state";
import type { App } from "./flow";
import type { RemotePlayers } from "./remote-players";

// Shared online lobby: while the player is in the lobby, report the local
// position 10 times a second and draw everyone else (lobby/remote-players.ts).
// The lobby has no rules, so its positions are client-reported; the server
// bounds them (cmd/server/lobby_proto.go). Self-contained: a 100 ms timer
// opens the socket in the lobby and closes it everywhere else.
const SEND_MS = 100;
const RETRY_MS = 5000;
const MIN_FACING_SPEED = 0.2;
const HEADER = 8;

export function startPresence(app: App, remotes: RemotePlayers): void {
  let ws: WebSocket | null = null;
  let retryAt = 0;
  let sentName = "";
  let facing = 0;
  const buf = new Uint8Array(PRESENCE_LEN);
  const view = new DataView(buf.buffer);
  buf[0] = LobbyMsg.Presence;
  buf[1] = 1;

  const close = (): void => {
    if (!ws) return;
    ws.onclose = null;
    ws.onmessage = null;
    ws.close();
    ws = null;
    remotes.clear();
  };

  const open = (): void => {
    const s = new WebSocket(`${serverBase()}/lobby`);
    s.binaryType = "arraybuffer";
    s.onmessage = (m: MessageEvent<ArrayBuffer>) => receive(remotes, new DataView(m.data));
    s.onclose = () => { ws = null; remotes.clear(); retryAt = performance.now() + RETRY_MS; };
    ws = s;
    sentName = "";
  };

  const sendName = (s: WebSocket): void => {
    const name = displayName(profileStore().get().name) || t("Jugador", "Player");
    if (name === sentName) return;
    const bytes = new TextEncoder().encode(name).subarray(0, MAX_NAME_BYTES);
    const msg = new Uint8Array(HEADER + bytes.length);
    msg[0] = LobbyMsg.Name;
    msg[1] = 1;
    msg.set(bytes, HEADER);
    s.send(msg);
    sentName = name;
  };

  window.setInterval(() => {
    const here = app.inLobby && app.engine.loaded && !isOnline();
    if (!here) { close(); return; }
    if (!ws && performance.now() >= retryAt) open();
    if (ws?.readyState !== WebSocket.OPEN) return;
    const c = app.engine.peek().curr;
    const vx = c[Snap.P0VelX];
    const vz = c[Snap.P0VelX + 1]; // VelZ follows VelX (snapshot layout)
    if (Math.hypot(vx, vz) > MIN_FACING_SPEED) facing = Math.atan2(vx, vz);
    view.setFloat64(HEADER, c[Snap.P0X], true);
    view.setFloat64(HEADER + 8, c[Snap.P0Y], true);
    view.setFloat64(HEADER + 16, c[Snap.P0Z], true);
    view.setFloat64(HEADER + 24, facing, true);
    sendName(ws);
    ws.send(buf);
  }, SEND_MS);
}

function receive(remotes: RemotePlayers, v: DataView): void {
  if (v.byteLength < HEADER + 8 || v.getUint8(1) !== 1) return;
  const f = (i: number): number => v.getFloat64(HEADER + i * 8, true);
  switch (v.getUint8(0)) {
    case LobbyMsg.Roster: {
      const n = Math.min(f(0) | 0, Math.floor((v.byteLength - HEADER - 8) / (ROSTER_STRIDE * 8)));
      for (let i = 0; i < n; i++) {
        const k = 1 + i * ROSTER_STRIDE;
        remotes.place(f(k), f(k + 1), f(k + 2), f(k + 3), f(k + 4));
      }
      break;
    }
    case LobbyMsg.PlayerName:
      remotes.name(f(0), new TextDecoder().decode(new Uint8Array(v.buffer, HEADER + 8)));
      break;
    case LobbyMsg.Gone:
      remotes.remove(f(0));
      break;
    default:
      break;
  }
}
