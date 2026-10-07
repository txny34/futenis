import type { Engine } from "../engine";
import { LobbyCfg } from "../lobby/pads";
import { CFG_TRAIN_FIRST, CFG_TRAIN_LAST } from "../train-config";
import { el, slider } from "../ui/dom";

// ?dev=1 live-tuning panel (contact-v2 §13). Indices mirror the scalar order
// of sim/config_layout.go (cfgGravity = 0 ... cfgMode = 44, cfgServeTimeLimit
// = 45, Dash 46..49, scoring 50..54, Entrenamiento 55..71 (train-config.ts),
// Lobby 72..74 (lobby/pads.ts); parts table from 75, CfgLen = 152). Applied between
// frames via engine.setConfig; never touched by the frame loop.
interface Param { readonly idx: number; readonly label: string; readonly min: number; readonly max: number; readonly step: number }

// Every min/max below is audited against two sources of truth: the field's
// real unit/sign (see sim/config.go's doc comments) and sim/config_layout.go's
// Config.Sanitize clamp range -- a default that fell outside its own slider
// range, or a slider that let the user pick a value Sanitize immediately
// clamps away, both counted as bugs here (code-review finding #2). Gravity
// was the broken one: Config.Gravity is positive (defaults to 9.81, applied
// on -Y by physics.go) and Sanitize bounds it to [minGravity=0.1,
// maxGravity=50], but the slider offered -20..0 -- every value in its range
// was invalid and got silently clamped to 0.1 the moment Sanitize ran.
const PARAMS: readonly Param[] = [
  { idx: 0, label: "Gravedad", min: 1, max: 25, step: 0.1 },
  { idx: 1, label: "Rebote piso", min: 0, max: 1, step: 0.01 },
  { idx: 2, label: "Rebote pared", min: 0, max: 1, step: 0.01 },
  { idx: 5, label: "Vel. jugador", min: 0, max: 12, step: 0.1 },
  { idx: 6, label: "Vel. sprint", min: 0, max: 15, step: 0.1 },
  { idx: 10, label: "Vel. salto", min: 0, max: 10, step: 0.1 },
  { idx: 15, label: "Carga máx (s)", min: 0.1, max: 2, step: 0.05 },
  { idx: 17, label: "Buffer golpe (s)", min: 0, max: 0.5, step: 0.01 },
  { idx: 26, label: "Globo apex mín", min: 1, max: 12, step: 0.1 },
  { idx: 27, label: "Globo apex máx", min: 1, max: 15, step: 0.1 },
  { idx: 28, label: "Margen red drive", min: 0, max: 2, step: 0.05 },
  { idx: 31, label: "Calidad perfecta ≤", min: 0, max: 1, step: 0.01 },
  { idx: 32, label: "Calidad buena ≤", min: 0, max: 1, step: 0.01 },
  { idx: 33, label: "Desvío por potencia", min: 0, max: 1, step: 0.01 },
  { idx: 40, label: "Alcance patada", min: 0.2, max: 3, step: 0.05 },
  { idx: 41, label: "Patada vel. mín", min: 1, max: 30, step: 0.5 },
  { idx: 42, label: "Patada vel. máx", min: 1, max: 40, step: 0.5 },
  { idx: 46, label: "Dash vel.", min: 0, max: 25, step: 0.1 },
  { idx: 47, label: "Dash duración (s)", min: 0.02, max: 0.6, step: 0.01 },
  { idx: 49, label: "Dash cooldown (s)", min: 0, max: 3, step: 0.05 },
  // Lobby (docs/design/lobby.md §3), ranges = Sanitize clamps.
  { idx: LobbyCfg.HoldTime, label: "Lobby: espera pad (s)", min: 0.3, max: 5, step: 0.05 },
  { idx: LobbyCfg.HalfX, label: "Lobby: medio largo (m)", min: 10, max: 30, step: 0.5 },
  { idx: LobbyCfg.HalfZ, label: "Lobby: medio ancho (m)", min: 8, max: 25, step: 0.5 },
];

const CFG_POINTS_TO_WIN = 43;
const CFG_MODE = 44;
/** cfgScoringMode..cfgFirstServer: match format set by configure(), kept on reset. */
const CFG_MATCH_FORMAT: readonly number[] = [50, 51, 52, 53, 54];
const PANEL_STYLE =
  "position:fixed;top:8px;right:8px;width:300px;max-height:90vh;overflow:auto;z-index:50;" +
  "background:rgba(13,17,23,.92);padding:8px;border-radius:8px;font:12px system-ui;color:#e6edf3";
const fmt = (v: number): string => v.toFixed(2);

export function isDevMode(): boolean {
  return new URLSearchParams(window.location.search).get("dev") === "1";
}

/** Builds (or rebuilds) the panel from the live Config. Call after load and after configure(). */
export function mountDevPanel(engine: Engine, onError: (msg: string) => void): () => void {
  const root = el("aside", "fx-dev", "", { "aria-label": "Panel dev" });
  root.style.cssText = PANEL_STYLE;
  document.body.append(root);
  let defaults: Float64Array | null = null;

  const apply = (cfg: Float64Array): void => {
    try { engine.setConfig(cfg); } catch (err) { onError(err instanceof Error ? err.message : String(err)); }
  };

  const rebuild = (): void => {
    let cfg: Float64Array;
    try { cfg = engine.getConfig(); } catch (err) { onError(err instanceof Error ? err.message : String(err)); return; }
    if (!defaults) defaults = cfg.slice();
    const reset = el("button", "fx-btn fx-btn--ghost", "Reset defaults", { type: "button" });
    reset.addEventListener("click", () => {
      if (!defaults) return;
      const next = defaults.slice();
      next[CFG_POINTS_TO_WIN] = cfg[CFG_POINTS_TO_WIN]; // keep the current match setup
      next[CFG_MODE] = cfg[CFG_MODE];
      for (const i of CFG_MATCH_FORMAT) next[i] = cfg[i];
      for (let i = CFG_TRAIN_FIRST; i <= CFG_TRAIN_LAST; i++) next[i] = cfg[i]; // menu-owned ball machine
      apply(next);
      rebuild();
    });
    const rows = PARAMS.map((p) => slider({
      label: p.label, min: p.min, max: p.max, step: p.step, value: cfg[p.idx], format: fmt,
      onInput: (v) => { cfg[p.idx] = v; apply(cfg); },
    }));
    root.replaceChildren(el("strong", "", "Tuning (dev)"), ...rows, reset);
  };

  rebuild();
  return rebuild;
}
