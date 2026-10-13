import type { CharacterStyle } from "../character/character";
import { BUILD_RANGE, EYEWEAR, FACIAL_HAIR, HAIR_STYLES, HEADWEAR, HEAD_SIZE_RANGE, sanitizeStyle } from "../character/style";
import { loadStyle, onStyleChange } from "../profile/style-store";
import { HEADER, Msg, writeHeader } from "./protocol";

// Online looks (cmd/server/internal/match/style.go): each client sends its
// own STYLE after WELCOME and on every Estilos edit; the server relays every
// seat's look (STYLES, server slot order). A look is 9 bytes: [present, hair,
// headwear, skin, hairColor, hatColor, headSize, build, shirt number + 1,
// eyewear, eyewearColor, facialHair, wristColor + 1 (0 = no wristbands),
// bootsColor]; the sizes are 0..255 over their range, number 0 = unknown (an
// old client: its missing bytes are zero, i.e. no accessories). The
// lobby relays the same bytes (lobby/presence.ts). Visual only: received
// bytes are untrusted and sanitized.

export const STYLE_LEN = 14;
const SEATS = 4;
export const STYLE_MSG_LEN = HEADER + STYLE_LEN;
export const STYLES_MSG_LEN = HEADER + SEATS * STYLE_LEN;
const SEAT_NUMBERS = [9, 7, 11, 5] as const;

/** A look per view slot (slot 0 = me); null = a bot or an empty seat. */
export type Looks = readonly (CharacterStyle | null)[];

const toByte = (v: number, [lo, hi]: readonly [number, number]): number => Math.round(((v - lo) / (hi - lo)) * 255);
const fromByte = (b: number, [lo, hi]: readonly [number, number]): number => lo + (b / 255) * (hi - lo);

/** STYLE (match) or LOBBY_STYLE (type) with my look. */
export function encodeStyle(st: Readonly<CharacterStyle>, type: number = Msg.Style): Uint8Array<ArrayBuffer> {
  const b = new Uint8Array(STYLE_MSG_LEN);
  writeHeader(new DataView(b.buffer), type);
  b.set([1, HAIR_STYLES.indexOf(st.hair), HEADWEAR.indexOf(st.headwear), st.skin, st.hairColor, st.hatColor,
    toByte(st.headSize, HEAD_SIZE_RANGE), toByte(st.build, BUILD_RANGE), st.number + 1,
    EYEWEAR.indexOf(st.eyewear), st.eyewearColor, FACIAL_HAIR.indexOf(st.facialHair), st.wristbands ? st.wristColor + 1 : 0, st.bootsColor], HEADER);
  return b;
}

/** Look sizes a server may relay: v2 (9 bytes, no accessories) and v3 (STYLE_LEN). */
export const LOOK_SIZES: readonly number[] = [9, STYLE_LEN];

/** The look at msg[o..o+size), or null when absent; fallbackNumber when it carries none.
 * Bytes past `size` (an older peer or server) read as 0: no accessories. */
export function decodeLook(src: Uint8Array, o: number, fallbackNumber: number, size = STYLE_LEN): CharacterStyle | null {
  if (src[o] === 0) return null;
  const msg = new Uint8Array(STYLE_LEN);
  msg.set(src.subarray(o, o + Math.min(size, STYLE_LEN)));
  o = 0;
  const n = msg[o + 8];
  return sanitizeStyle({
    hair: HAIR_STYLES[msg[o + 1]], headwear: HEADWEAR[msg[o + 2]], skin: msg[o + 3], hairColor: msg[o + 4],
    hatColor: msg[o + 5], headSize: fromByte(msg[o + 6], HEAD_SIZE_RANGE), build: fromByte(msg[o + 7], BUILD_RANGE),
    number: n > 0 ? n - 1 : fallbackNumber,
    eyewear: EYEWEAR[msg[o + 9]], eyewearColor: msg[o + 10], facialHair: FACIAL_HAIR[msg[o + 11]],
    wristbands: msg[o + 12] > 0, wristColor: Math.max(0, msg[o + 12] - 1), bootsColor: msg[o + 13],
  });
}

/** STYLES in my view order: the client in server slot `me` sees slot s at s ^ me (game/view.go). */
export function decodeStyles(msg: Uint8Array, len: number, me: number): Looks | null {
  const size = (len - HEADER) / SEATS;
  if (!LOOK_SIZES.includes(size)) return null;
  const looks: (CharacterStyle | null)[] = [null, null, null, null];
  for (let s = 0; s < SEATS; s++) looks[s ^ me] = decodeLook(msg, HEADER + s * size, SEAT_NUMBERS[s], size);
  return looks;
}

/** One match's look exchange: sends mine once seated (and on edits), reports everyone's. */
export class StyleLink {
  onLooks: (looks: Looks) => void = () => undefined;
  private me = -1; // my server slot; -1 until WELCOME
  private readonly unsubscribe: () => void;

  constructor(private readonly send: (msg: Uint8Array<ArrayBuffer>) => void) {
    this.unsubscribe = onStyleChange((st) => { if (this.me >= 0) this.send(encodeStyle(st)); });
  }

  /** WELCOME (also after a rejoin): my seat, then my look. */
  welcome(slot: number): void {
    this.me = slot & (SEATS - 1);
    this.send(encodeStyle(loadStyle()));
  }

  receive(msg: Uint8Array, len: number): void {
    const looks = this.me >= 0 ? decodeStyles(msg, len, this.me) : null;
    if (looks) this.onLooks(looks);
  }

  dispose(): void { this.unsubscribe(); }
}
