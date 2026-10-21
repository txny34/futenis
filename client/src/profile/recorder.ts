import { EventKind, Quality, SimEvent } from "../snapshot";
import { pushHistory } from "./history";
import { missions } from "./missions";
import { applyMatch, applyPerfect, applyTraining, BotDifficulty } from "./progress";
import { profileWriter, Queue } from "./store";

// Puente eventos del sim -> perfil local + misiones. Lo alimentan GameLoop.onGameEvent
// (cada evento no-lobby) y lobby/flow.ts (inicio de modo, fin de partido, salida del
// entrenamiento). Sin allocations en el camino caliente: los eventos que no cuentan
// salen en la primera comparación; los que cuentan son pocos por partido.
type Activity = Queue | "training" | null;

const HUMAN_SLOT = 0;
/** Un entrenamiento cuenta como completado con al menos esta cantidad de devoluciones válidas. */
export const TRAINING_MIN_RETURNS = 5;

class Recorder {
  private activity: Activity = null;
  private bot: BotDifficulty = "normal";
  private ranked = false;
  private returns = 0;
  private run = 0;
  private bestRun = 0;

  /** Arranca un partido (cola 1v1/2v2) o un entrenamiento. */
  begin(activity: Queue | "training", bot: BotDifficulty, ranked: boolean): void {
    this.activity = activity;
    this.bot = bot;
    this.ranked = activity !== "training" && ranked;
    this.returns = 0;
    this.run = 0;
    this.bestRun = 0;
  }

  onEvent(e: SimEvent): void {
    if (this.activity === null) return;
    if (e.kind === EventKind.Kick) {
      if (e.player === HUMAN_SLOT && e.quality === Quality.Perfect) this.perfect();
    } else if (e.kind === EventKind.ReturnOK && this.activity === "training") {
      this.returns++;
      this.run++;
      if (this.run > this.bestRun) {
        this.bestRun = this.run;
        missions().record("trainStreak", this.run);
      }
    } else if (e.kind === EventKind.ReturnFail) {
      this.run = 0;
    }
  }

  /** Fin de un partido con puntuación (GameLoop.onMatchOver); games = juegos local/visita. */
  /** Records the result; returns the queue's ELO before/after for ranked matches (rank note), else null. */
  matchOver(won: boolean, games0: number, games1: number, at = Date.now()): { before: number; after: number } | null {
    const a = this.activity;
    if (a !== "1v1" && a !== "2v2") return null;
    this.activity = null;
    const w = profileWriter();
    const before = w.get().elo[a];
    w.update((s) => applyMatch(s, a, this.bot, won, this.ranked));
    pushHistory({ at, queue: a, ranked: this.ranked, won, games0, games1, eloDelta: w.get().elo[a] - before });
    missions().record("match", 1);
    if (won) missions().record("win", 1);
    return this.ranked ? { before, after: w.get().elo[a] } : null;
  }

  /** Salida del modo actual sin resultado (volver al lobby, revancha, otro modo). */
  end(): void {
    const wasTraining = this.activity === "training";
    this.activity = null;
    if (!wasTraining || this.returns < TRAINING_MIN_RETURNS) return;
    const best = this.bestRun;
    profileWriter().update((s) => applyTraining(s, best));
    missions().record("training", 1);
  }

  private perfect(): void {
    profileWriter().update(applyPerfect);
    missions().record("perfect", 1);
  }
}

let instance: Recorder | null = null;
export function recorder(): Recorder {
  if (instance === null) instance = new Recorder();
  return instance;
}
