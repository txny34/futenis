import { t } from "../i18n/t";
import { CM, FACING_Q, LobbyMsg, MAX_NAME_BYTES, PRESENCE_LEN, ROSTER_Q_HEAD, ROSTER_Q_STRIDE, ROSTER_STRIDE } from "../net/lobby-proto";
import { serverBase } from "../net/server-url";
import { displayName } from "../profile/display-name";
import { profileStore } from "../profile/store";
import { Extra, Snap } from "../snapshot";
import { isOnline } from "./online-state";
import type { App } from "./flow";
import type { RemotePlayers } from "./remote-players";

// Shared online lobby: while the player is in the lobby, report the local
// position 10 times a second and draw everyone else (lobby/remote-players.ts).
// The lobby has no rules, so its positions are client-reported; the server
// bounds them (cmd/server/internal/wire/lobby.go). Self-contained: a 100 ms timer
// opens the socket in the lobby and closes it everywhere else.
const SEND_MS = 100;
const RETRY_MIN_MS = 2000;
const RETRY_MAX_MS = 60000;
const NAME_RUNES = 16; // the server keeps 16 runes; cut on runes, never mid UTF-8
const encoder = new TextEncoder();
const decoder = new TextDecoder();
const MIN_FACING_SPEED = 0.2;
const HEADER = 8;

/** Lobby actions that need the socket (moderation). */
export interface Presence { report(id: number): void }

export function startPresence(app: App, remotes: RemotePlayers): Presence {
  let ws: WebSocket | null = null;
  let retryAt = 0;
  let backoff = RETRY_MIN_MS;
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
    const s = new WebSocket(`${serverBase()}/lobby?v=2`); // compact roster (ROSTER_Q)
    s.binaryType = "arraybuffer";
    s.onmessage = (m: MessageEvent<ArrayBuffer>) => receive(remotes, new DataView(m.data));
    s.onopen = () => { backoff = RETRY_MIN_MS; };
    s.onclose = () => { // server down or full: back off (with jitter so clients don't reconnect in sync)
      ws = null;
      remotes.clear();
      retryAt = performance.now() + backoff * (0.5 + Math.random());
      backoff = Math.min(RETRY_MAX_MS, backoff * 2);
    };
    ws = s;
    sentName = "";
  };

  const sendName = (s: WebSocket): void => {
    const name = displayName(profileStore().get().name) || t("Jugador", "Player");
    if (name === sentName) return;
    const bytes = encoder.encode([...name].slice(0, NAME_RUNES).join("")).subarray(0, MAX_NAME_BYTES);
    const msg = new Uint8Array(HEADER + bytes.length);
    msg[0] = LobbyMsg.Name;
    msg[1] = 1;
    msg.set(bytes, HEADER);
    s.send(msg);
    sentName = name;
  };

  window.setInterval(() => {
    // Hidden tab: timers slow to ~1 Hz, so leave instead of stuttering for others.
    const here = app.inLobby && app.engine.loaded && !isOnline() && !document.hidden;
    if (!here) { close(); return; }
    if (!ws && performance.now() >= retryAt) open();
    if (ws?.readyState !== WebSocket.OPEN) return;
    const c = app.engine.peek().curr;
    const vx = c[Snap.P0VelX + Extra.VelX];
    const vz = c[Snap.P0VelX + Extra.VelZ];
    if (Math.hypot(vx, vz) > MIN_FACING_SPEED) facing = Math.atan2(vx, vz);
    view.setFloat64(HEADER, c[Snap.P0X], true);
    view.setFloat64(HEADER + 8, c[Snap.P0Y], true);
    view.setFloat64(HEADER + 16, c[Snap.P0Z], true);
    view.setFloat64(HEADER + 24, facing, true);
    sendName(ws);
    ws.send(buf);
  }, SEND_MS);
  return {
    report(id) {
      if (ws?.readyState !== WebSocket.OPEN) return;
      const msg = new Uint8Array(HEADER + 8);
      msg[0] = LobbyMsg.Report;
      msg[1] = 1;
      new DataView(msg.buffer).setFloat64(HEADER, id, true);
      ws.send(msg);
    },
  };
}

function receive(remotes: RemotePlayers, v: DataView): void {
  if (v.byteLength < HEADER + 4 || v.getUint8(1) !== 1) return; // each case checks its own length
  const f = (i: number): number => v.getFloat64(HEADER + i * 8, true);
  const type = v.getUint8(0);
  if (type !== LobbyMsg.RosterQ && v.byteLength < HEADER + 8) return; // float messages start with an f64
  switch (type) {
    case LobbyMsg.Roster: {
      const n = Math.min(f(0) | 0, Math.floor((v.byteLength - HEADER - 8) / (ROSTER_STRIDE * 8)));
      for (let i = 0; i < n; i++) {
        const k = 1 + i * ROSTER_STRIDE;
        remotes.place(f(k), f(k + 1), f(k + 2), f(k + 3), f(k + 4));
      }
      break;
    }
    case LobbyMsg.RosterQ: {
      const n = Math.min(v.getUint32(HEADER, true), Math.floor((v.byteLength - ROSTER_Q_HEAD) / ROSTER_Q_STRIDE));
      for (let i = 0; i < n; i++) {
        const o = ROSTER_Q_HEAD + i * ROSTER_Q_STRIDE;
        remotes.place(v.getUint32(o, true), v.getInt16(o + 4, true) * CM, v.getUint16(o + 6, true) * CM,
          v.getInt16(o + 8, true) * CM, v.getInt16(o + 10, true) * FACING_Q);
      }
      break;
    }
    case LobbyMsg.PlayerName:
      remotes.name(f(0), decoder.decode(new Uint8Array(v.buffer, HEADER + 8)));
      break;
    case LobbyMsg.Gone:
      remotes.remove(f(0));
      break;
    default:
      break;
  }
}
