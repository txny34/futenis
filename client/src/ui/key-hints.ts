import { DEFAULT_KEYS, type KeyAction, type KeyBinds, keyLabel } from "./keybinds";

// Current key labels for help texts (HUD, action chips, tutorial, info panel).
// main.ts applySettings pushes the binds; readers call these at build time.
let current: KeyBinds = DEFAULT_KEYS;

export function setHintKeys(k: KeyBinds): void { current = k; }

export function keyHint(a: KeyAction): string { return keyLabel(current[a]); }

/** "WASD" for the default set, else the four labels in up/left/down/right order. */
export function moveHint(): string {
  const parts = [keyHint("up"), keyHint("left"), keyHint("down"), keyHint("right")];
  return parts.every((p) => p.length === 1) ? parts.join("") : parts.join(" ");
}
