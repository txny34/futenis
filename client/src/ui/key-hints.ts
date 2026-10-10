import { DEFAULT_KEYS, type KeyAction, type KeyBinds, keyLabel } from "./keybinds";
import { t } from "../i18n/t";

// Current key labels for help texts (HUD, action chips, tutorial, info panel).
// main.ts applySettings pushes the binds; readers call these at build time.
let current: KeyBinds = DEFAULT_KEYS;

export function setHintKeys(k: KeyBinds): void { current = k; }

// keybinds.ts can't import t() itself (circular import through settings.ts),
// so named key labels (e.g. "Espacio") are translated here instead.
const KEY_NAME: Readonly<Record<string, string>> = {
  "Espacio": t("Espacio", "Space"), "Shift izq.": t("Shift izq.", "Left Shift"), "Shift der.": t("Shift der.", "Right Shift"),
  "Ctrl izq.": t("Ctrl izq.", "Left Ctrl"), "Ctrl der.": t("Ctrl der.", "Right Ctrl"),
  "Alt izq.": t("Alt izq.", "Left Alt"), "Alt der.": t("Alt der.", "Right Alt"),
  "Bloq Mayús": t("Bloq Mayús", "Caps Lock"), "Borrar": t("Borrar", "Backspace"),
};

/** keyLabel() (Spanish names) in the UI language. */
export function keyName(code: string): string {
  const label = keyLabel(code);
  return KEY_NAME[label] ?? label;
}

export function keyHint(a: KeyAction): string { return keyName(current[a]); }

/** "WASD" for the default set, else the four labels in up/left/down/right order. */
export function moveHint(): string {
  const parts = [keyHint("up"), keyHint("left"), keyHint("down"), keyHint("right")];
  return parts.every((p) => p.length === 1) ? parts.join("") : parts.join(" ");
}
