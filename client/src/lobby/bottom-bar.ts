import { button, el } from "../ui/dom";
import type { Difficulty, GameMode } from "../ui/types";

// Lobby bottom bar (docs/design/lobby.md §7): difficulty chips · JUGAR ·
// Entrenamiento ⚙ · Ajustes, plus the "Pará sobre un pad" hint. DOM only,
// touched on clicks and mode changes, never per frame.
const DIFFICULTY_KEY = "futenis.difficulty";
const LAST_MODE_KEY = "futenis.lastMode";
const DEFAULT_DIFFICULTY: Difficulty = "normal";
const DEFAULT_MODE: GameMode = "1v1";
const HINT_TEXT = "Pará sobre un pad para jugar";
const DIFFICULTIES: ReadonlyArray<readonly [Difficulty, string]> = [
  ["easy", "Fácil"], ["normal", "Normal"], ["hard", "Difícil"], ["pro", "Pro"],
];
const MODES: readonly GameMode[] = ["1v1", "2v2", "playground"];

function read(key: string): string | null {
  try { return window.localStorage.getItem(key); } catch { return null; }
}

function write(key: string, value: string): void {
  try { window.localStorage.setItem(key, value); } catch { /* storage blocked: keep the in-memory value */ }
}

const isDifficulty = (v: string | null): v is Difficulty => DIFFICULTIES.some(([d]) => d === v);
const isMode = (v: string | null): v is GameMode => MODES.some((m) => m === v);

export interface BarActions {
  play: (mode: GameMode) => void;
  training: () => void;
  settings: () => void;
}

export class BottomBar {
  readonly root = el("div", "fx-lobby");
  private readonly hint = el("p", "fx-lobby__hint", HINT_TEXT, { role: "status" });
  private readonly chipEls: HTMLButtonElement[] = [];
  private diff: Difficulty;
  private last: GameMode;

  constructor(parent: HTMLElement, a: BarActions) {
    const d = read(DIFFICULTY_KEY);
    const m = read(LAST_MODE_KEY);
    this.diff = isDifficulty(d) ? d : DEFAULT_DIFFICULTY;
    this.last = isMode(m) ? m : DEFAULT_MODE;
    const bar = el("nav", "fx-lobby__bar", "", { "aria-label": "Lobby" });
    bar.append(
      this.chips(),
      button("JUGAR", () => a.play(this.last), "fx-btn fx-btn--hero fx-lobby__play"),
      button("Entrenamiento ⚙", a.training, "fx-btn fx-lobby__btn"),
      button("Ajustes", a.settings, "fx-btn fx-btn--ghost fx-lobby__btn"),
    );
    this.root.append(this.hint, bar);
    this.root.hidden = true;
    parent.append(this.root);
  }

  get difficulty(): Difficulty { return this.diff; }

  set visible(v: boolean) { this.root.hidden = !v; }

  /** First EvPadEnter: the player found the pads, the hint goes away for good. */
  dismissHint(): void { this.hint.hidden = true; }

  /** Remembers the mode JUGAR repeats (localStorage futenis.lastMode). */
  remember(mode: GameMode): void {
    this.last = mode;
    write(LAST_MODE_KEY, mode);
  }

  /** Mirrors a difficulty picked elsewhere (pad setup screen). */
  setDifficulty(d: Difficulty): void {
    this.diff = d;
    write(DIFFICULTY_KEY, d);
    this.chipEls.forEach((c, i) => c.setAttribute("aria-pressed", String(DIFFICULTIES[i][0] === d)));
  }

  private chips(): HTMLElement {
    const group = el("div", "fx-chips fx-lobby__diff", "", { role: "group", "aria-label": "Dificultad de los bots" });
    for (const [value, label] of DIFFICULTIES) {
      const b = el("button", "fx-chip", label, { type: "button", "aria-pressed": String(value === this.diff) });
      b.addEventListener("click", () => {
        this.setDifficulty(value);
        b.blur(); // keyboard Space must keep jumping, not re-press the chip
      });
      this.chipEls.push(b);
      group.append(b);
    }
    return group;
  }
}
