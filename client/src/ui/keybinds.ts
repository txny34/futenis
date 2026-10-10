// Remappable keyboard actions (backlog "Accesibilidad: remapeo de teclas").
// Mouse buttons (golpe/globo) and the mode keys B/R stay fixed.
export type KeyAction = "up" | "down" | "left" | "right" | "jump" | "dash" | "free";

export type KeyBinds = Readonly<Record<KeyAction, string>>;

export const KEY_ACTIONS: readonly (readonly [KeyAction, string])[] = [
  ["up", "Adelante"], ["down", "Atrás"], ["left", "Izquierda"], ["right", "Derecha"],
  ["jump", "Salto"], ["dash", "Dash"], ["free", "Mouse libre"],
];

export const DEFAULT_KEYS: KeyBinds = {
  up: "KeyW", down: "KeyS", left: "KeyA", right: "KeyD", jump: "Space", dash: "KeyQ", free: "ControlLeft",
};

/** Esc = menus; B/R = launch ball / reset (main.ts). Never bindable. */
export const RESERVED_KEYS: ReadonlySet<string> = new Set(["Escape", "KeyB", "KeyR"]);
// Allowlist: letters, digits, numpad digits, arrows, modifiers and a few named keys.
// F-keys, Meta, ContextMenu etc. keep their browser/OS actions, so they are never bindable.
const CODE_RE = /^(Key[A-Z]|Digit\d|Numpad\d|Arrow(Up|Down|Left|Right)|(Shift|Control|Alt)(Left|Right)|Space|Tab|CapsLock|Enter|Backspace)$/;

export function isBindable(code: string): boolean {
  return CODE_RE.test(code) && !RESERVED_KEYS.has(code);
}

/** Field-by-field validation; a duplicate or invalid key falls back to the whole default set. */
export function validateKeys(raw: unknown): KeyBinds {
  if (typeof raw !== "object" || raw === null) return DEFAULT_KEYS;
  const r = raw as Record<string, unknown>;
  const out: Record<string, string> = {};
  for (const [a] of KEY_ACTIONS) {
    const v = r[a];
    out[a] = typeof v === "string" && isBindable(v) ? v : DEFAULT_KEYS[a];
  }
  const keys = KEY_ACTIONS.map(([a]) => out[a]);
  return new Set(keys).size === keys.length ? (out as KeyBinds) : DEFAULT_KEYS;
}

/** Binds action to code; an action already on that code takes the old key (swap). */
export function rebind(k: KeyBinds, action: KeyAction, code: string): KeyBinds {
  const other = KEY_ACTIONS.find(([a]) => a !== action && k[a] === code)?.[0];
  return other ? { ...k, [action]: code, [other]: k[action] } : { ...k, [action]: code };
}

const NAMED: Readonly<Record<string, string>> = {
  Space: "Espacio", ShiftLeft: "Shift izq.", ShiftRight: "Shift der.", ControlLeft: "Ctrl izq.", ControlRight: "Ctrl der.",
  AltLeft: "Alt izq.", AltRight: "Alt der.", ArrowUp: "↑", ArrowDown: "↓", ArrowLeft: "←", ArrowRight: "→",
  Tab: "Tab", CapsLock: "Bloq Mayús", Enter: "Enter", Backspace: "Borrar",
};

export function keyLabel(code: string): string {
  if (code.startsWith("Key")) return code.slice(3);
  if (code.startsWith("Digit")) return code.slice(5);
  if (code.startsWith("Numpad")) return `Num ${code.slice(6)}`;
  return NAMED[code] ?? code;
}
