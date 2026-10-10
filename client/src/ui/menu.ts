/** MenuUI: settings, Entrenamiento setup and pause overlays over the lobby/match. Owns Esc + arrow navigation.
 * No title or main menu: the game boots straight into the lobby (docs/design/lobby.md §7). */
import { button, el } from "./dom";
import { panel, pauseMenu } from "./screens";
import { lobbyMenu, LobbyMenuModel } from "./lobby-menu";
import { loadSettings, saveSettings, Settings } from "./settings";
import { settingsView } from "./settings-view";
import { reportView } from "./report-view";
import { trainingView } from "./training-view";
import type { TrainingSettings } from "./training-settings";
import { t } from "../i18n/t";

export type { Difficulty, GameMode, MatchOptions } from "./types";
export type { Settings } from "./settings";

type Screen = "setup" | "settings" | "report" | "pause" | "lobby" | "hidden";
type Listener<T> = (v: T) => void;

const NAV_SELECTOR = "textarea, button:not(:disabled), input, [tabindex='0']";
const KEY_NEXT = new Set(["ArrowDown"]);
const KEY_PREV = new Set(["ArrowUp"]);
const ESC_DEBOUNCE_MS = 150; // one Esc can both drop pointer lock and arrive as keydown

export class MenuUI {
  private current: Settings = loadSettings();
  private screen: Screen = "hidden";
  private inGame = false;
  private lobbyModel: (() => LobbyMenuModel) | null = null;
  private settingsBack: Screen = "hidden";
  private reportText: ((note: string) => string) | null = null;
  private reportBack: Screen = "hidden";
  private openedAt = -Infinity; // last pause/lobby-menu open, for the Esc debounce
  private readonly openCbs: Listener<boolean>[] = [];
  private readonly settingsCbs: Listener<Settings>[] = [];
  private readonly pauseCbs: Listener<boolean>[] = [];
  private readonly quitCbs: Listener<void>[] = [];
  private readonly layer: HTMLElement;

  constructor(private readonly root: HTMLElement) {
    this.layer = el("div", "fx-menu", "", { role: "dialog", "aria-modal": "true" });
    root.append(this.layer);
    window.addEventListener("keydown", (e) => this.onKey(e));
    this.render("hidden");
  }

  get settings(): Settings { return this.current; }
  get isOpen(): boolean { return this.screen !== "hidden"; }

  /** Fires on every overlay open/close (the lobby disables controls while a panel is up). */
  onOpenChange(cb: Listener<boolean>): void { this.openCbs.push(cb); }
  onSettingsChange(cb: Listener<Settings>): void { this.settingsCbs.push(cb); }
  /** Fires true when the pause overlay opens mid-match, false when play resumes. */
  onPauseChange(cb: Listener<boolean>): void { this.pauseCbs.push(cb); }
  /** Fires when the player leaves a match via "Volver al lobby" (pause). */
  onQuit(cb: Listener<void>): void { this.quitCbs.push(cb); }

  /** A match/training is live: Esc opens the pause overlay. */
  setInGame(v: boolean): void { this.inGame = v; }
  /** Items of the lobby menu (Esc in the lobby), read on every open. */
  setLobbyMenu(model: () => LobbyMenuModel): void { this.lobbyModel = model; }
  /** Lobby menu: opens from Esc (keydown or lost pointer lock) while no match is live. */
  openLobbyMenu(): void {
    if (this.inGame || !this.lobbyModel || this.isOpen) return;
    this.openedAt = performance.now();
    this.render("lobby");
  }
  /** Lobby menu "Ajustes": "Volver" returns to the lobby menu. */
  openSettings(): void {
    this.settingsBack = this.screen === "lobby" ? "lobby" : "hidden";
    this.render("settings");
  }
  /** Lobby menu "Entrenamiento": edit the ball machine; "Empezar" hands the settings back. */
  openTraining(onStart: (t: TrainingSettings) => void): void {
    const back: Screen = this.screen === "lobby" ? "lobby" : "hidden";
    this.mount("setup", trainingView((t) => { this.hide(); onStart(t); }, () => this.render(back)));
  }
  /** Builds the bug report text (main.ts owns the game context). */
  setReport(build: (note: string) => string): void { this.reportText = build; }
  /** "Reportar un bug" from the lobby menu or pause: "Volver" returns there. */
  openReport(): void {
    const build = this.reportText;
    if (!build) return;
    this.reportBack = this.screen === "pause" || this.screen === "lobby" ? this.screen : "hidden";
    this.mount("report", reportView(build, () => this.back()));
  }
  showPause(): void {
    if (!this.inGame) return;
    this.openedAt = performance.now();
    this.render("pause");
    this.emitPause(true);
  }
  hide(): void { this.render("hidden"); }

  private resume(): void { this.hide(); this.emitPause(false); }

  private quit(): void {
    this.inGame = false;
    this.hide();
    for (const cb of this.quitCbs) cb();
  }

  private emitPause(paused: boolean): void { for (const cb of this.pauseCbs) cb(paused); }

  private update(fn: (s: Settings) => Settings): void {
    this.current = fn(this.current);
    saveSettings(this.current);
    for (const cb of this.settingsCbs) cb(this.current);
  }

  private build(screen: Screen): HTMLElement | null {
    switch (screen) {
      case "pause": return pauseMenu({
        resume: () => this.resume(), settings: () => this.render("settings"), report: () => this.openReport(), quit: () => this.quit(),
      });
      case "settings": return this.settingsScreen();
      case "lobby": return this.lobbyModel ? lobbyMenu(this.lobbyModel(), () => this.hide()) : null;
      default: return null;
    }
  }

  private settingsScreen(): HTMLElement {
    const back = button(t("Volver", "Back"), () => this.back(), "fx-btn fx-btn--ghost");
    return panel(t("Ajustes", "Settings"), settingsView(() => this.current, (fn) => this.update(fn)), back);
  }

  private render(screen: Screen): void {
    this.mount(screen, this.build(screen));
  }

  private mount(screen: Screen, node: HTMLElement | null): void {
    this.screen = screen;
    this.layer.replaceChildren(...(node ? [node] : []));
    this.layer.hidden = node === null;
    this.root.classList.toggle("fx-root--ingame", true); // always over the live 3D scene (lobby or match)
    const first = this.layer.querySelector<HTMLElement>(NAV_SELECTOR);
    if (first) first.focus({ preventScroll: true });
    for (const cb of this.openCbs) cb(node !== null);
  }

  private back(): void {
    if (this.screen === "pause") this.resume();
    else if (this.screen === "settings") this.render(this.inGame ? "pause" : this.settingsBack);
    else if (this.screen === "report") this.backFromReport();
    else this.hide(); // setup, lobby menu
  }

  /** Pause reopens through render (already paused); a stray state falls back to resume. */
  private backFromReport(): void {
    if (this.reportBack === "hidden" && this.inGame) this.resume();
    else this.render(this.reportBack);
  }

  private onKey(e: KeyboardEvent): void {
    if (e.code === "Escape") {
      e.preventDefault();
      if (e.target instanceof HTMLTextAreaElement) { e.target.blur(); return; } // first Esc keeps the draft
      if ((this.screen === "lobby" || this.screen === "pause") && performance.now() - this.openedAt < ESC_DEBOUNCE_MS) return;
      if (this.isOpen) this.back();
      else if (this.inGame) this.showPause();
      else this.openLobbyMenu();
      return;
    }
    if (this.isOpen && (KEY_NEXT.has(e.code) || KEY_PREV.has(e.code))) this.moveFocus(e);
  }

  private moveFocus(e: KeyboardEvent): void {
    const target = e.target;
    if (target instanceof HTMLInputElement && target.type !== "checkbox") return; // sliders/radios use arrows
    if (target instanceof HTMLTextAreaElement) return; // arrows move the caret
    const items = Array.from(this.layer.querySelectorAll<HTMLElement>(NAV_SELECTOR));
    if (items.length === 0) return;
    e.preventDefault();
    const idx = target instanceof HTMLElement ? items.indexOf(target) : -1;
    const step = KEY_NEXT.has(e.code) ? 1 : -1;
    const next = items[(idx + step + items.length) % items.length];
    if (next) next.focus();
  }
}
