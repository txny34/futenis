/** Static screens: title, main menu, pre-match setup, pause. Pure builders. */
import { button, Choice, disabledButton, el, segmented } from "./dom";
import { DEFAULT_POINTS, Difficulty, GameMode, MatchOptions, POINTS_CHOICES } from "./types";

const DIFFICULTIES: readonly Choice<Difficulty>[] = [
  { value: "easy", label: "Fácil" }, { value: "normal", label: "Normal" },
  { value: "hard", label: "Difícil" }, { value: "pro", label: "Pro" },
];

const MODE_TITLES: Record<GameMode, string> = {
  "1v1": "1v1 vs IA", "2v2": "2v2 · vos + IA vs IA", playground: "Playground",
};

export function panel(title: string, ...children: HTMLElement[]): HTMLElement {
  const p = el("section", "fx-card");
  p.append(el("h2", "fx-card__title", title), ...children);
  return p;
}

export function titleScreen(onEnter: () => void): HTMLElement {
  const s = el("div", "fx-title");
  const logo = el("h1", "fx-logo");
  logo.append(el("span", "fx-logo__a", "FUT"), el("span", "fx-logo__ball", "", { "aria-hidden": "true" }),
    el("span", "fx-logo__b", "ENIS"));
  logo.setAttribute("aria-label", "FUTENIS");
  s.append(logo, el("p", "fx-tagline", "Un toque. Una pared. Cero excusas."),
    button("Jugar", onEnter, "fx-btn fx-btn--hero"));
  return s;
}

export interface MainActions {
  pick: (mode: GameMode) => void;
  settings: () => void;
}

export function mainMenu(a: MainActions): HTMLElement {
  const nav = el("nav", "fx-stack", "", { "aria-label": "Menú principal" });
  nav.append(
    button("1v1 vs IA", () => a.pick("1v1")),
    button("2v2 (vos + IA vs IA)", () => a.pick("2v2")),
    button("Playground — práctica libre", () => a.pick("playground")),
    button("Ajustes", a.settings, "fx-btn fx-btn--ghost"),
    disabledButton("Online", "pronto"),
  );
  return panel("Elegí cómo jugar", nav);
}

export function matchSetup(mode: GameMode, onGo: (o: MatchOptions) => void, onBack: () => void): HTMLElement {
  let difficulty: Difficulty = "normal";
  let pointsToWin = DEFAULT_POINTS;
  const pts = POINTS_CHOICES.map((p) => ({ value: p, label: String(p) }));
  const rows: HTMLElement[] = [
    segmented("Dificultad de la IA", DIFFICULTIES, difficulty, (d) => { difficulty = d; }),
  ];
  if (mode !== "playground") {
    rows.push(segmented("Puntos para ganar", pts, pointsToWin, (p) => { pointsToWin = p; }));
  }
  const actions = el("div", "fx-actions");
  actions.append(
    button("Volver", onBack, "fx-btn fx-btn--ghost"),
    button("¡A la cancha!", () => onGo({ mode, difficulty, pointsToWin }), "fx-btn fx-btn--hero"),
  );
  return panel(MODE_TITLES[mode], ...rows, actions);
}

export interface PauseActions { resume: () => void; settings: () => void; quit: () => void }

export function pauseMenu(a: PauseActions): HTMLElement {
  const nav = el("nav", "fx-stack", "", { "aria-label": "Pausa" });
  nav.append(
    button("Reanudar", a.resume, "fx-btn fx-btn--hero"),
    button("Ajustes", a.settings),
    button("Salir al menú", a.quit, "fx-btn fx-btn--ghost"),
  );
  return panel("Pausa", nav);
}
