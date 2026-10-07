import type { Engine, MatchSetup } from "../engine";
import { recorder } from "../profile/recorder";
import { profileStore } from "../profile/store";
import type { Hud } from "../hud";
import type { Controls } from "../input";
import type { GameLoop, LoopState, MatchResult } from "../loop";
import type { CourtView } from "../render";
import { EventKind, Mode, Snap } from "../snapshot";
import { applyTraining } from "../train-config";
import { MatchScreens, VS_INTRO_MS } from "../ui/match-screens";
import type { Difficulty, GameMode, MatchOptions, MenuUI, Settings } from "../ui/menu";
import { DEFAULT_TRAINING, loadTraining } from "../ui/training-settings";
import { DEFAULT_POINTS } from "../ui/types";
import type { BottomBar } from "./bottom-bar";
import { botLevelFor } from "./bot-level";
import type { ProfileCard } from "./profile-card";
import type { PlayModal } from "../ui/play-modal/play-modal";
import { PAD_MODES } from "./pads";

// Lobby <-> match flow (docs/design/lobby.md §6): the sim detects pads, the
// client decides. Every switch is configure() + reset() on the same wasm.
const LOADING_TEXT = "Cargando…";
const MODE_ID: Readonly<Record<GameMode, MatchSetup["mode"]>> = { "1v1": 0, "2v2": 1, playground: 2 };
const LOBBY_MODE_ID: MatchSetup["mode"] = Mode.Lobby;
const LOBBY_BOT: Difficulty = "normal"; // the lobby has no bot; any valid id
const DIFFICULTY_ID: Readonly<Record<Difficulty, number>> = { easy: 0, normal: 1, hard: 2, pro: 3 };
// Single local human on slot 0 in every mode: rivals/partner are AI (filled inside wasm).
const HUMAN_MASK = 1;
const HUMAN_TEAM = 0;
const CFG_DASH_COOLDOWN = 49; // sim/config_layout.go cfgDashCooldownTime
const TEAM_SIZE: Readonly<Record<GameMode, number>> = { "1v1": 1, "2v2": 2, playground: 1 };
const INTRO_SUBTITLE = "Un set a 3 juegos";
const YAW_LOBBY = -Math.PI / 2; // spawn faces -Z, toward the pads
const YAW_COURT = 0; // behind the human, toward the rival (+X)

export const errText = (err: unknown): string => (err instanceof Error ? err.message : String(err));

export interface App {
  engine: Engine;
  view: CourtView;
  hud: Hud;
  controls: Controls;
  state: LoopState;
  bar: BottomBar;
  profile: ProfileCard;
  screens: MatchScreens;
  menu: MenuUI;
  /** JUGAR / 1v1-2v2 pads: the "Elegí un modo" modal. */
  play: PlayModal;
  /** null while in the lobby. */
  mode: GameMode | null;
  inLobby: boolean;
  pending: MatchOptions | null;
  refreshDev: (() => void) | null;
  loop: GameLoop | null;
  last: MatchOptions | null;
}

/** "Marcador de pique": siempre / solo Fácil / nunca. */
export function landingAllowed(s: Settings, o: MatchOptions | null): boolean {
  const pref = s.game.landingMarker;
  return pref === "always" || (pref === "easy" && (o === null || o.difficulty === "easy" || o.mode === "playground"));
}

function setup(mode: MatchSetup["mode"], difficulty: Difficulty, pointsToWin: number): MatchSetup {
  return { mode, difficulty: DIFFICULTY_ID[difficulty], pointsToWin, humanMask: HUMAN_MASK };
}

/** Shared tail of entering any mode: fresh loop, live controls, HUD reset. */
function arm(app: App, lobby: boolean): void {
  app.engine.reset();
  app.inLobby = lobby;
  app.state.running = true;
  app.state.paused = false;
  app.controls.releaseAll();
  app.controls.enabled = !app.menu.isOpen && !app.play.isOpen;
  app.loop?.rearm();
  app.hud.resetMatch();
  app.screens.hide();
  app.view.setLobby(lobby);
  app.view.rig.face(lobby ? YAW_LOBBY : YAW_COURT);
  app.bar.visible = lobby;
  app.profile.visible = lobby;
  app.menu.setInGame(!lobby);
  app.refreshDev?.();
}

/** Spawns the guest in the lobby (boot, "Volver al lobby"). */
export function enterLobby(app: App): void {
  app.view.setLobby(true);
  app.bar.visible = true;
  app.profile.visible = true;
  if (!app.engine.loaded) return; // load() calls back once wasm is up
  app.hud.error("");
  recorder().end();
  app.engine.clearInputs();
  app.engine.configure(setup(LOBBY_MODE_ID, LOBBY_BOT, DEFAULT_POINTS));
  app.mode = null;
  app.state.holdUntil = 0;
  app.state.showLanding = false;
  app.hud.setTrainingOnDemand(false);
  arm(app, true);
}

export function startMatch(app: App, o: MatchOptions): void {
  if (!app.engine.loaded) {
    app.pending = o; // queued until wasm finishes loading
    app.hud.error(LOADING_TEXT);
    return;
  }
  app.hud.error("");
  app.engine.clearInputs();
  app.engine.configure(setup(MODE_ID[o.mode], o.difficulty, o.pointsToWin));
  const training = o.training ?? DEFAULT_TRAINING; // ball machine: live Config, kept by reset()
  if (o.mode === "playground") applyTraining(app.engine, training);
  app.mode = o.mode;
  app.last = o;
  recorder().end();
  recorder().begin(o.mode === "playground" ? "training" : o.mode, o.difficulty, o.ranked);
  app.state.showLanding = landingAllowed(app.menu.settings, o);
  app.hud.setTrainingOnDemand(o.mode === "playground" && training.onDemand);
  app.hud.setDashCooldown(app.engine.getConfig()[CFG_DASH_COOLDOWN]);
  arm(app, false);
  app.controls.lock();
  if (o.mode !== "playground") {
    app.state.holdUntil = performance.now() + VS_INTRO_MS;
    app.screens.showIntro(TEAM_SIZE[o.mode], INTRO_SUBTITLE);
  } else {
    app.state.holdUntil = 0;
  }
}

/** Options for a mode picked in the lobby; the bot level follows the queue's ELO. */
export function lobbyOptions(mode: GameMode, ranked = false, training = loadTraining()): MatchOptions {
  const difficulty = mode === "playground" ? LOBBY_BOT : botLevelFor(profileStore().get().elo[mode]);
  return { mode, difficulty, ranked: ranked && mode !== "playground", pointsToWin: DEFAULT_POINTS, training: mode === "playground" ? training : undefined };
}

export function safeStart(app: App, o: MatchOptions): void {
  try { startMatch(app, o); } catch (err) { app.hud.error(errText(err)); }
}

/** EvPadEnter/Leave/Start from the loop. */
export function onPadEvent(app: App, kind: number, pad: number): void {
  if (!app.inLobby) return;
  if (kind === EventKind.PadEnter) {
    app.view.lobby.pulse(pad);
  } else if (kind === EventKind.PadStart) {
    const mode = PAD_MODES[pad];
    if (!mode) return;
    app.view.lobby.flash(pad);
    openPadSetup(app, mode);
  }
}

/** A pad opens its overlay (controls off, pointer free): Entrenamiento -> training panel,
 * 1v1/2v2 -> "Elegí un modo" with that casual card preselected. The sim re-fires
 * EvPadStart only after leaving and re-entering the pad, so closing is safe. */
function openPadSetup(app: App, mode: GameMode): void {
  if (app.menu.isOpen || app.play.isOpen) return;
  if (mode === "playground") app.menu.openTraining((t) => safeStart(app, lobbyOptions("playground", false, t)));
  else app.play.open({ mode, ranked: false });
}

export function showResult(app: App, r: Readonly<MatchResult>, snap: Float64Array): void {
  app.controls.enabled = false;
  app.controls.releaseAll();
  const winner = r.winner >= 0 ? r.winner : snap[Snap.Sets0] >= snap[Snap.Sets1] ? 0 : 1;
  recorder().matchOver(winner === HUMAN_TEAM, r.games0, r.games1);
  app.screens.showResult({ won: winner === HUMAN_TEAM, games0: r.games0, games1: r.games1 }, {
    rematch: () => { if (app.last) safeStart(app, app.last); },
    lobby: () => {
      try { enterLobby(app); } catch (err) { app.hud.error(errText(err)); }
    },
  });
}
