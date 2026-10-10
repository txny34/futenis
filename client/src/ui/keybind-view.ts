import { button, el } from "./dom";
import { DEFAULT_KEYS, isBindable, KEY_ACTIONS, type KeyAction, type KeyBinds, keyLabel, rebind } from "./keybinds";

// Ajustes > Controles: click a key, press the new one (Esc cancels). Taking a
// key another action uses swaps them. Mouse actions are listed but fixed.
const MOUSE_ROWS: readonly (readonly [string, string])[] = [
  ["Cámara / apuntar", "Mouse"], ["Zoom", "Rueda"], ["Golpe", "Click izq."], ["Globo", "Click der."],
  ["Lanzar bola / reiniciar", "B / R"],
];
const WAITING = "Presioná una tecla…";
const RESERVED_MSG = "Esa tecla no se puede usar (Esc, B, R, F1–F12, Windows…). Probá otra.";

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
      ...KEY_ACTIONS.map(([action, label]) => row(label, keyButton(action, label))),
      ...MOUSE_ROWS.map(([label, key]) => row(label, el("kbd", "fx-kbd", key))),
    );
    if (focus) buttons.get(focus)?.focus({ preventScroll: true });
  };
  const commit = (k: KeyBinds, focus?: KeyAction): void => { keys = k; save(k); render(focus); };

  function keyButton(action: KeyAction, label: string): HTMLButtonElement {
    const b = button(keyLabel(keys[action]), () => capture(action), "fx-kbd fx-kbd--btn");
    b.setAttribute("aria-label", `${label}: ${keyLabel(keys[action])}, cambiar tecla`);
    buttons.set(action, b);
    return b;
  }

  function capture(action: KeyAction): void {
    render(action); // restores any other key still waiting
    const b = buttons.get(action);
    if (!b) return;
    b.textContent = WAITING;
    status.textContent = `${WAITING} (Esc cancela)`;
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
  const reset = button("Restablecer teclas", () => commit(DEFAULT_KEYS), "fx-btn fx-btn--ghost");
  return [el("h3", "fx-sub", "Controles"), list, status, reset];
}

function row(label: string, key: HTMLElement): HTMLElement {
  const li = el("li", "fx-binds__row");
  li.append(el("span", "", label), key);
  return li;
}
