import { missions } from "../profile/missions";
import { button, el } from "../ui/dom";
import { icon, IconName } from "./icons";
import { MissionsPanel } from "./missions-panel";
import { t } from "../i18n/t";

// Lobby bottom bar (docs/design/lobby.md §7 + "Perfil local y misiones"):
// dock [Equipo · Estilos · Misiones]  JUGAR  [Regalo · Tienda · Mercancía]. JUGAR opens
// "Elegí un modo" (ui/play-modal); Entrenamiento and Ajustes live in the Esc lobby menu. DOM only,
// touched on clicks and mode/mission changes, never per frame.
const SOON_TEXT = t("Próximamente", "Coming soon");
const SOON_MS = 1800;
const LEFT: ReadonlyArray<readonly [IconName, string]> = [
  ["team", t("Equipo", "Team")], ["styles", t("Estilos", "Styles")], ["missions", t("Misiones", "Missions")],
];
const RIGHT: ReadonlyArray<readonly [IconName, string]> = [
  ["gift", t("Regalo", "Gift")], ["shop", t("Tienda", "Shop")], ["merch", t("Mercancía", "Merch")],
];

export interface BarActions {
  play: () => void;
}

export class BottomBar {
  readonly root = el("div", "fx-lobby");
  private readonly toast = el("p", "fx-lobby__toast", SOON_TEXT, { role: "status" });
  private readonly badge = el("span", "fx-lobby__badge");
  private readonly panel: MissionsPanel;
  private toastTimer = 0;

  constructor(parent: HTMLElement, a: BarActions) {
    this.panel = new MissionsPanel(parent);
    const bar = el("nav", "fx-lobby__bar", "", { "aria-label": "Lobby" });
    bar.append(...LEFT.map(([i, l]) => this.dockButton(i, l)), button(t("JUGAR", "PLAY"), a.play, "fx-lobby__play"));
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
