import { displayName } from "../profile/display-name";
import { profileStore } from "../profile/store";
import type { Difficulty, GameMode } from "../ui/types";
import { type IntroLabels, VS_INTRO_MS } from "../ui/match-screens";
import type { App } from "./flow";
import { t } from "../i18n/t";

// Match intro (backlog "Animación de entrada"): VS card with who plays,
// camera sweep into the shoulder view, skippable with any game key or click.
const INTRO_SUBTITLE = t("Al mejor de 3 sets · sets a 5", "Best of 3 sets · sets to 5");
const MS_PER_S = 1000;
const BOT_LABEL: Readonly<Record<Difficulty, string>> = {
  easy: t("Fácil", "Easy"), normal: "Normal", hard: t("Difícil", "Hard"), pro: "Pro",
};
// Esc keeps opening pause; modifiers/system keys (alt-tab, screenshots) never skip.
const NO_SKIP_KEYS = new Set(["Escape", "AltLeft", "AltRight", "ControlLeft", "ControlRight", "ShiftLeft", "ShiftRight",
  "MetaLeft", "MetaRight", "Tab", "PrintScreen"]);

let cancel: (() => void) | null = null;

function labels(mode: Exclude<GameMode, "playground">, difficulty: Difficulty): IntroLabels {
  const p = profileStore().get();
  const bot = BOT_LABEL[difficulty];
  return mode === "1v1"
    ? { left: `${displayName(p.name)} · ${p.elo["1v1"]} ELO`, right: `Bot · ${bot}` }
    : { left: `${displayName(p.name)} + bot · ${p.elo["2v2"]} ELO`, right: `Bots · ${bot}` };
}

const reducedMotion = (): boolean => window.matchMedia("(prefers-reduced-motion: reduce)").matches;

/** Drops a running intro's listeners and camera sweep (lobby, new match). */
export function cancelIntro(): void {
  cancel?.();
}

/** Holds the sim for the intro; the first game key/click (not the one that started the match) skips it. */
export function playIntro(app: App, mode: Exclude<GameMode, "playground">, members: number, difficulty: Difficulty): void {
  cancelIntro();
  app.state.holdUntil = performance.now() + VS_INTRO_MS;
  app.screens.showIntro(members, INTRO_SUBTITLE, labels(mode, difficulty));
  if (!reducedMotion()) app.view.rig.intro(VS_INTRO_MS / MS_PER_S);
  const ac = new AbortController();
  const timer = window.setTimeout(() => ac.abort(), VS_INTRO_MS);
  const stop = (): void => {
    window.clearTimeout(timer);
    ac.abort();
    app.view.rig.intro(0);
    if (cancel === stop) cancel = null;
  };
  const skip = (e: Event): void => {
    if (e instanceof KeyboardEvent && (e.repeat || NO_SKIP_KEYS.has(e.code))) return;
    if (e.target instanceof Element && e.target.closest("[role='dialog']")) return; // pause/menu clicks
    e.preventDefault();
    e.stopImmediatePropagation(); // capture phase: the skip press never reaches the controls (no toss/strike)
    stop();
    app.state.holdUntil = 0;
    app.screens.hide();
  };
  cancel = stop;
  window.setTimeout(() => { // next task: the click/key that started the match has already fired
    if (ac.signal.aborted) return;
    window.addEventListener("keydown", skip, { capture: true, signal: ac.signal });
    window.addEventListener("pointerdown", skip, { capture: true, signal: ac.signal });
  }, 0);
}
