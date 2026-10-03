/** MenuUI: title, main menu, match setup, settings and pause overlay. Owns Esc + arrow navigation. */
import { button, el } from "./dom";
import { mainMenu, matchSetup, panel, pauseMenu, titleScreen } from "./screens";
import { loadSettings, saveSettings, Settings } from "./settings";
import { settingsView } from "./settings-view";
import { GameMode, MatchOptions } from "./types";

export type { Difficulty, GameMode, MatchOptions } from "./types";
export type { Settings } from "./settings";

type Screen = "title" | "main" | "setup" | "settings" | "pause" | "hidden";
type Listener<T> = (v: T) => void;

const NAV_SELECTOR = "button:not(:disabled), input, [tabindex='0']";
const KEY_NEXT = new Set(["ArrowDown"]);
const KEY_PREV = new Set(["ArrowUp"]);

export class MenuUI {
  private current: Settings = loadSettings();
  private screen: Screen = "hidden";
  private inGame = false;
  private readonly startCbs: Listener<MatchOptions>[] = [];
  private readonly settingsCbs: Listener<Settings>[] = [];
  private readonly pauseCbs: Listener<boolean>[] = [];
  private readonly quitCbs: Listener<void>[] = [];
  private readonly layer: HTMLElement;

  constructor(private readonly root: HTMLElement) {
    this.layer = el("div", "fx-menu", "", { role: "dialog", "aria-modal": "true" });
    root.append(this.layer);
    window.addEventListener("keydown", (e) => this.onKey(e));
    this.render("title");
  }

  get settings(): Settings { return this.current; }
  get isOpen(): boolean { return this.screen !== "hidden"; }

  onStart(cb: Listener<MatchOptions>): void { this.startCbs.push(cb); }
  onSettingsChange(cb: Listener<Settings>): void { this.settingsCbs.push(cb); }
  /** Fires true when the pause overlay opens mid-match, false when play resumes. */
  onPauseChange(cb: Listener<boolean>): void { this.pauseCbs.push(cb); }
  /** Fires when the player leaves a match via "Salir al menú". */
  onQuit(cb: Listener<void>): void { this.quitCbs.push(cb); }

  showMain(): void { this.inGame = false; this.render("main"); }
  showPause(): void {
    if (!this.inGame) return;
    this.render("pause");
    this.emitPause(true);
  }
  hide(): void { this.render("hidden"); }

  private resume(): void { this.hide(); this.emitPause(false); }

  private start(o: MatchOptions): void {
    this.inGame = true;
    this.hide();
    for (const cb of this.startCbs) cb(o);
  }

  private quit(): void {
    this.showMain();
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
      case "title": return titleScreen(() => this.render("main"));
      case "main": return mainMenu({ pick: (m) => this.setup(m), settings: () => this.render("settings") });
      case "pause": return pauseMenu({
        resume: () => this.resume(), settings: () => this.render("settings"), quit: () => this.quit(),
      });
      case "settings": return this.settingsScreen();
      default: return null;
    }
  }

  private settingsScreen(): HTMLElement {
    const back = button("Volver", () => this.back(), "fx-btn fx-btn--ghost");
    return panel("Ajustes", settingsView(() => this.current, (fn) => this.update(fn)), back);
  }

  private setup(mode: GameMode): void {
    this.mount("setup", matchSetup(mode, (o) => this.start(o), () => this.render("main")));
  }

  private render(screen: Screen): void {
    this.mount(screen, this.build(screen));
  }

  private mount(screen: Screen, node: HTMLElement | null): void {
    this.screen = screen;
    this.layer.replaceChildren(...(node ? [node] : []));
    this.layer.hidden = node === null;
    this.root.classList.toggle("fx-root--ingame", this.inGame);
    const first = this.layer.querySelector<HTMLElement>(NAV_SELECTOR);
    if (first) first.focus({ preventScroll: true });
  }

  private back(): void {
    if (this.screen === "pause") this.resume();
    else if (this.screen === "settings") this.render(this.inGame ? "pause" : "main");
    else if (this.screen === "setup") this.render("main");
  }

  private onKey(e: KeyboardEvent): void {
    if (e.code === "Escape") {
      e.preventDefault();
      if (this.isOpen) this.back();
      else this.showPause();
      return;
    }
    if (this.isOpen && (KEY_NEXT.has(e.code) || KEY_PREV.has(e.code))) this.moveFocus(e);
  }

  private moveFocus(e: KeyboardEvent): void {
    const target = e.target;
    if (target instanceof HTMLInputElement && target.type !== "checkbox") return; // sliders/radios use arrows
    const items = Array.from(this.layer.querySelectorAll<HTMLElement>(NAV_SELECTOR));
    if (items.length === 0) return;
    e.preventDefault();
    const idx = target instanceof HTMLElement ? items.indexOf(target) : -1;
    const step = KEY_NEXT.has(e.code) ? 1 : -1;
    const next = items[(idx + step + items.length) % items.length];
    if (next) next.focus();
  }
}
