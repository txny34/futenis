import { missions } from "../profile/missions";
import { button, el } from "../ui/dom";
import type { GameMode } from "../ui/types";
import { icon, IconName } from "./icons";
import { MissionsPanel } from "./missions-panel";

// Lobby bottom bar (docs/design/lobby.md §7 + "Perfil local y misiones"):
// dock [Equipo · Estilos · Misiones]  JUGAR  [Regalo · Tienda · Mercancía]. Difficulty,
// Entrenamiento and Ajustes live in the Esc lobby menu (ui/lobby-menu.ts). DOM only,
// touched on clicks and mode/mission changes, never per frame.
const LAST_MODE_KEY = "futenis.lastMode";
const DEFAULT_MODE: GameMode = "1v1";
const MODES: readonly GameMode[] = ["1v1", "2v2", "playground"];
const SOON_TEXT = "Próximamente";
const SOON_MS = 1800;
const LEFT: ReadonlyArray<readonly [IconName, string]> = [["team", "Equipo"], ["styles", "Estilos"], ["missions", "Misiones"]];
const RIGHT: ReadonlyArray<readonly [IconName, string]> = [["gift", "Regalo"], ["shop", "Tienda"], ["merch", "Mercancía"]];

function read(key: string): string | null {
  try { return window.localStorage.getItem(key); } catch { return null; }
}

function write(key: string, value: string): void {
  try { window.localStorage.setItem(key, value); } catch { /* storage blocked: keep the in-memory value */ }
}

const isMode = (v: string | null): v is GameMode => MODES.some((m) => m === v);

export interface BarActions {
  play: (mode: GameMode) => void;
}

export class BottomBar {
  readonly root = el("div", "fx-lobby");
  private readonly toast = el("p", "fx-lobby__toast", SOON_TEXT, { role: "status" });
  private readonly badge = el("span", "fx-lobby__badge");
  private readonly panel: MissionsPanel;
  private toastTimer = 0;
  private last: GameMode;

  constructor(parent: HTMLElement, a: BarActions) {
    const m = read(LAST_MODE_KEY);
    this.last = isMode(m) ? m : DEFAULT_MODE;
    this.panel = new MissionsPanel(parent);
    const bar = el("nav", "fx-lobby__bar", "", { "aria-label": "Lobby" });
    bar.append(...LEFT.map(([i, l]) => this.dockButton(i, l)), button("JUGAR", () => a.play(this.last), "fx-lobby__play"));
    bar.append(...RIGHT.map(([i, l]) => this.dockButton(i, l)));
    this.toast.hidden = true;
    this.root.append(this.toast, bar);
    this.root.hidden = true;
    parent.append(this.root);
    missions().onChange(() => this.refreshBadge());
    this.refreshBadge();
  }

  set visible(v: boolean) {
    this.root.hidden = !v;
    this.panel.visible = v;
    if (v) this.refreshBadge(); // a midnight/Monday reset may have happened in a match
  }

  /** Remembers the mode JUGAR repeats (localStorage futenis.lastMode). */
  remember(mode: GameMode): void {
    this.last = mode;
    write(LAST_MODE_KEY, mode);
  }

  private dockButton(name: IconName, label: string): HTMLButtonElement {
    const b = el("button", "fx-lobby__icon", "", { type: "button" });
    b.append(icon(name), el("span", "fx-lobby__label", label));
    if (name === "missions") b.append(this.badge);
    b.addEventListener("click", () => {
      b.blur();
      if (name === "missions") this.panel.collapsed = !this.panel.collapsed;
      else this.soon();
    });
    return b;
  }

  private refreshBadge(): void {
    const n = missions().claimableCount();
    this.badge.textContent = String(n);
    this.badge.hidden = n === 0;
  }

  private soon(): void {
    this.toast.hidden = false;
    window.clearTimeout(this.toastTimer);
    this.toastTimer = window.setTimeout(() => { this.toast.hidden = true; }, SOON_MS);
  }
}
