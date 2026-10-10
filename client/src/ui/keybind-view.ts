import { button, el } from "./dom";
import { DEFAULT_KEYS, isBindable, KEY_ACTIONS, type KeyAction, type KeyBinds, rebind } from "./keybinds";
import { t } from "../i18n/t";
import { keyName } from "./key-hints";

// Ajustes > Controles: click a key, press the new one (Esc cancels). Taking a
// key another action uses swaps them. Mouse actions are listed but fixed.
const MOUSE_ROWS: readonly (readonly [string, string])[] = [
  [t("Cámara / apuntar", "Camera / aim"), t("Mouse", "Mouse")], [t("Zoom", "Zoom"), t("Rueda", "Wheel")],
  [t("Golpe", "Strike"), t("Click izq.", "Left click")], [t("Globo", "Lob"), t("Click der.", "Right click")],
  [t("Lanzar bola / reiniciar", "Launch ball / reset"), "B / R"],
];
const WAITING = t("Presioná una tecla…", "Press a key…");
const CAPTURING = t("Presioná una tecla… (Esc cancela)", "Press a key… (Esc cancels)");
const RESERVED_MSG = t("Esa tecla no se puede usar (Esc, B, R, F1–F12, Windows…). Probá otra.", "That key can't be used (Esc, B, R, F1–F12, Windows…). Try another.");

// keybinds.ts can't import the t() helper itself (it would create a circular
// import through settings.ts), so action/key-name labels are translated here.
const ACTION_LABEL: Readonly<Record<string, string>> = {
  "Adelante": t("Adelante", "Forward"), "Atrás": t("Atrás", "Back"), "Izquierda": t("Izquierda", "Left"),
  "Derecha": t("Derecha", "Right"), "Salto": t("Salto", "Jump"), "Mouse libre": t("Mouse libre", "Free mouse"),
};
const trAction = (label: string): string => ACTION_LABEL[label] ?? label;

export function keybindView(initial: KeyBinds, save: (k: KeyBinds) => void): HTMLElement[] {
  let keys = initial;
  let stopCapture: (() => void) | null = null;
  const list = el("ul", "fx-binds");
  const status = el("p", "fx-note", "", { role: "status" });

  const buttons = new Map<KeyAction, HTMLButtonElement>();
  /** Rebuilds the rows; `focus` re-focuses that action's key (replaceChildren drops focus). */
  const render = (focus?: KeyAction): void => {
    stopCapture?.();
    buttons.clear();
    list.replaceChildren(
      ...KEY_ACTIONS.map(([action, label]) => row(trAction(label), keyButton(action, trAction(label)))),
      ...MOUSE_ROWS.map(([label, key]) => row(label, el("kbd", "fx-kbd", key))),
    );
    if (focus) buttons.get(focus)?.focus({ preventScroll: true });
  };
  const commit = (k: KeyBinds, focus?: KeyAction): void => { keys = k; save(k); render(focus); };

  function keyButton(action: KeyAction, label: string): HTMLButtonElement {
    const b = button(keyName(keys[action]), () => capture(action), "fx-kbd fx-kbd--btn");
    b.setAttribute("aria-label", `${label}: ${keyName(keys[action])}, ${t("cambiar tecla", "change key")}`);
    buttons.set(action, b);
    return b;
  }

  function capture(action: KeyAction): void {
    render(action); // restores any other key still waiting
    const b = buttons.get(action);
    if (!b) return;
    b.textContent = WAITING;
    status.textContent = CAPTURING;
    const onKey = (e: KeyboardEvent): void => {
      if (!list.isConnected) { stopCapture?.(); return; } // panel closed while waiting: let the key through
      e.preventDefault();
      e.stopImmediatePropagation(); // the menu must not see this Esc/arrow
      if (e.code === "Escape") { status.textContent = ""; render(action); return; }
      if (!isBindable(e.code)) { status.textContent = RESERVED_MSG; return; }
      status.textContent = "";
      commit(rebind(keys, action, e.code), action);
    };
    window.addEventListener("keydown", onKey, true);
    stopCapture = (): void => { window.removeEventListener("keydown", onKey, true); stopCapture = null; };
  }

  render();
  const reset = button(t("Restablecer teclas", "Reset keys"), () => commit(DEFAULT_KEYS), "fx-btn fx-btn--ghost");
  return [el("h3", "fx-sub", t("Controles", "Controls")), list, status, reset];
}

function row(label: string, key: HTMLElement): HTMLElement {
  const li = el("li", "fx-binds__row");
  li.append(el("span", "", label), key);
  return li;
}
