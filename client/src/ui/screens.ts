/** Static screens: shared panel + pause. Pure builders. */
import { button, el } from "./dom";

export function panel(title: string, ...children: HTMLElement[]): HTMLElement {
  const p = el("section", "fx-card");
  p.append(el("h2", "fx-card__title", title), ...children);
  return p;
}

export interface PauseActions { resume: () => void; settings: () => void; quit: () => void }

export function pauseMenu(a: PauseActions): HTMLElement {
  const nav = el("nav", "fx-stack", "", { "aria-label": "Pausa" });
  nav.append(
    button("Reanudar", a.resume, "fx-btn fx-btn--hero"),
    button("Ajustes", a.settings),
    button("Volver al lobby", a.quit, "fx-btn fx-btn--ghost"),
  );
  return panel("Pausa", nav);
}
