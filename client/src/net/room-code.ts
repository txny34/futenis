// Private room codes (cmd/server/hub.go roomCode: ^[A-Z0-9]{3,12}$). No 0/O/1/I
// so a code read aloud or typed from a phone is unambiguous.
const CODE_LEN = 5;
const CODE_CHARS = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789";
const ROOM_RE = /^[A-Z0-9]{3,12}$/;

export function randomRoomCode(): string {
  let s = "";
  for (const b of crypto.getRandomValues(new Uint8Array(CODE_LEN))) s += CODE_CHARS[b % CODE_CHARS.length];
  return s;
}

/** A valid room code from user input or the URL, else null. */
export function parseRoomCode(raw: string | null): string | null {
  const code = (raw ?? "").trim().toUpperCase();
  return ROOM_RE.test(code) ? code : null;
}

/** Shareable link that opens the game straight into the room. */
export const roomLink = (code: string): string => `${location.origin}${location.pathname}?room=${code}`;
