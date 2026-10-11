import { HEADER, Msg, writeHeader } from "./protocol";

// Online session states and the server's close codes (cmd/server/internal/wire/close.go).
export type NetStatus =
  | "conectando"
  | "esperando rival"
  | "jugando"
  | "el rival se fue"
  | "desconectado"
  | "versión distinta, recargá"
  | "desconectado por inactividad"
  | "nadie se unió a la sala"
  | "demasiados mensajes"
  | "iniciá sesión"
  | "reconectando"
  | "rival reconectando";

export function authMessage(token: string): Uint8Array<ArrayBuffer> {
  const body = new TextEncoder().encode(token);
  const msg = new Uint8Array(new ArrayBuffer(HEADER + body.length));
  writeHeader(new DataView(msg.buffer), Msg.Auth);
  msg.set(body, HEADER);
  return msg;
}

export const CLOSE_STATUS: Readonly<Record<number, NetStatus | undefined>> = {
  4003: "desconectado por inactividad",
  4004: "demasiados mensajes",
  4005: "nadie se unió a la sala",
  4006: "iniciá sesión",
};

/** cmd/server closeGone: the held seat is gone (match over or the hold ran out). */
export const CLOSE_GONE = 4007;
/** cmd/server closeRequeue: ranked matchmaking moves a lone waiter to a closer room; queue again at once. */
export const CLOSE_REQUEUE = 4009;

/** Same server, ?rejoin=TOKEN instead of the room/queue (cmd/server/hub_rejoin.go). */
export function rejoinUrl(url: string, token: string): string {
  const u = new URL(url);
  u.search = new URLSearchParams({ rejoin: token }).toString();
  return u.toString();
}

export const NORMAL_CLOSE = 1000;
// A dropped connection retries ?rejoin= inside the server's hold (cmd/server rejoinGrace 20 s).
export const REJOIN_WINDOW_MS = 18_000;
export const REJOIN_EVERY_MS = 1500;
export const REJOINABLE: ReadonlySet<NetStatus> = new Set<NetStatus>(["jugando", "rival reconectando", "reconectando"]);

const TOKEN = /^[0-9a-f]{32}$/;

/** SEAT body -> rejoin token (null if malformed). */
export function rejoinToken(body: Uint8Array): string | null {
  const s = new TextDecoder().decode(body);
  return TOKEN.test(s) ? s : null;
}
