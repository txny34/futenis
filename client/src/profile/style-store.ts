import { CharacterStyle, DEFAULT_STYLE } from "../character/character";
import { sanitizeStyle } from "../character/style";

// The player's own character style (Estilos), kept in this browser. Reads are
// sanitized (storage is untrusted); storage failures (private mode, blocked
// site data) fall back to the default and never throw.

const KEY = "futenis.style";
const listeners: Array<(s: Readonly<CharacterStyle>) => void> = [];
let current: Readonly<CharacterStyle> | null = null;

export function loadStyle(): Readonly<CharacterStyle> {
  if (current) return current;
  let raw: unknown = null;
  try {
    const text = localStorage.getItem(KEY);
    raw = text ? JSON.parse(text) : null;
  } catch {
    raw = null; // unreadable or blocked storage: default look
  }
  current = raw === null ? DEFAULT_STYLE : sanitizeStyle(raw);
  return current;
}

export function saveStyle(style: Readonly<CharacterStyle>): void {
  current = sanitizeStyle(style);
  try {
    localStorage.setItem(KEY, JSON.stringify(current));
  } catch {
    // storage blocked: the style still applies for this session
  }
  for (const f of listeners) f(current);
}

export function onStyleChange(f: (s: Readonly<CharacterStyle>) => void): void {
  listeners.push(f);
}
