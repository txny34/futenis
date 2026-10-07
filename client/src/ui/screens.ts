/** Static screens: pre-match setup (lobby pads) and pause. Pure builders. */
import { button, Choice, el, segmented } from "./dom";
import { DEFAULT_POINTS, Difficulty, GameMode, MatchOptions } from "./types";

const DIFFICULTIES: readonly Choice<Difficulty>[] = [
  { value: "easy", label: "Fácil" }, { value: "normal", label: "Normal" },
  { value: "hard", label: "Difícil" }, { value: "pro", label: "Pro" },
];

const MODE_TITLES: Record<GameMode, string> = {
  "1v1": "1v1 vs IA", "2v2": "2v2 · vos + IA vs IA", playground: "Entrenamiento",
};

export function panel(title: string, ...children: HTMLElement[]): HTMLElement {
  const p = el("section", "fx-card");
  p.append(el("h2", "fx-card__title", title), ...children);
  return p;
}

export interface SetupActions {
  go: (o: MatchOptions) => void;
  back: () => void;
  /** Every difficulty pick (the lobby bar mirrors it). */
  difficulty: (d: Difficulty) => void;
}

export function matchSetup(mode: GameMode, initial: Difficulty, a: SetupActions): HTMLElement {
  let difficulty = initial;
  const pointsToWin = DEFAULT_POINTS; // legacy simple scoring only; 1v1/2v2 play real padel (wasm configure)
  const rows: HTMLElement[] = [];
  if (mode !== "playground") { // playground: no AI rival, no scoring
    rows.push(
      segmented("Dificultad de la IA", DIFFICULTIES, difficulty, (d) => { difficulty = d; a.difficulty(d); }),
      el("p", "fx-note", "Pádel: 0 · 15 · 30 · 40 · ventaja — un set a 3 juegos."),
    );
  }
  const actions = el("div", "fx-actions");
  actions.append(
    button("Volver", a.back, "fx-btn fx-btn--ghost"),
    button("¡A la cancha!", () => a.go({ mode, difficulty, pointsToWin }), "fx-btn fx-btn--hero"),
  );
  return panel(MODE_TITLES[mode], ...rows, actions);
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
