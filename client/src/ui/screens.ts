/** Static screens: shared panel + pause. Pure builders. */
import { button, el } from "./dom";
import { t } from "../i18n/t";

export function panel(title: string, ...children: HTMLElement[]): HTMLElement {
  const p = el("section", "fx-card");
  p.append(el("h2", "fx-card__title", title), ...children);
  return p;
}

export interface PauseActions { resume: () => void; settings: () => void; report: () => void; quit: () => void }

export function pauseMenu(a: PauseActions): HTMLElement {
  const nav = el("nav", "fx-stack", "", { "aria-label": t("Pausa", "Pause") });
  nav.append(
    button(t("Reanudar", "Resume"), a.resume, "fx-btn fx-btn--hero"),
    button(t("Ajustes", "Settings"), a.settings),
    button(t("Reportar un bug", "Report a bug"), a.report),
    button(t("Volver al lobby", "Back to lobby"), a.quit, "fx-btn fx-btn--ghost"),
  );
  return panel(t("Pausa", "Pause"), nav);
}
