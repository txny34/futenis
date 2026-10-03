/** Entrenamiento setup panel: PISTA / GOLPE / SESIÓN cards + Volver / Empezar. Pure builder. */
import { applyThemeCss } from "../env/theme";
import { button, Choice, el, segmented, slider } from "./dom";
import { range2 } from "./range2";
import { minimap } from "./training-minimap";
import {
  FEED_DEPTH, INTERVAL, LATERAL, loadTraining, MACHINE_DEPTH, PERCENT, saveTraining,
  SHOT_DRIVE, SHOT_LOB, SHOT_SMASH, TrainingSettings,
} from "./training-settings";

type Patch = Partial<TrainingSettings>;

const SIDES: readonly [string, string] = ["Izquierda", "Derecha"];
const DEPTHS: readonly [string, string] = ["Red", "Fondo"];
const SHOT_CHIPS: readonly Choice<number>[] = [
  { value: SHOT_DRIVE, label: "Derecha" }, { value: SHOT_LOB, label: "Globo" }, { value: SHOT_SMASH, label: "Remate" },
];
const FEEDING: readonly Choice<boolean>[] = [{ value: false, label: "Continua" }, { value: true, label: "A demanda" }];
const BOUNCE: readonly Choice<boolean>[] = [{ value: false, label: "Permitido" }, { value: true, label: "Solo volea" }];
const VALID: readonly Choice<1 | 2>[] = [{ value: 1, label: "1.er bote" }, { value: 2, label: "2.º bote" }];
const RANDOM_HINT = "Cada bola toma un valor al azar dentro del rango. Acotalo para repetir la misma bola.";

const meters = (v: number): string => `${v.toFixed(1)} m`;
const percent = (v: number): string => `${Math.round(v)}%`;
const seconds = (v: number): string => `${v.toFixed(1)} s`;

function card(title: string, ...children: HTMLElement[]): HTMLElement {
  const c = el("section", "fx-tcard");
  c.append(el("h3", "fx-tcard__title", title), ...children);
  return c;
}

/** Multi-select chips over a bitmask; at least one chip always stays on. */
function chips(label: string, choices: readonly Choice<number>[], mask: number, onChange: (m: number) => void): HTMLElement {
  let current = mask;
  const group = el("div", "fx-chips", "", { role: "group", "aria-label": label });
  for (const c of choices) {
    const b = el("button", "fx-chip", c.label, { type: "button", "aria-pressed": String((mask & c.value) !== 0) });
    b.addEventListener("click", () => {
      const next = current ^ c.value;
      if (next === 0) return;
      current = next;
      b.setAttribute("aria-pressed", String((current & c.value) !== 0));
      onChange(current);
    });
    group.append(b);
  }
  const r = el("div", "fx-row");
  r.append(el("span", "fx-row__label", label), group);
  return r;
}

/** onStart receives the final settings (already persisted). */
export function trainingView(onStart: (t: TrainingSettings) => void, onBack: () => void): HTMLElement {
  let t = loadTraining();
  const map = minimap();
  const interval = el("div", "fx-tinterval");
  const set = (p: Patch): void => {
    t = { ...t, ...p };
    saveTraining(t);
    map.update(t);
    interval.hidden = t.onDemand;
  };

  const pista = card("Pista",
    el("div", "fx-tmap"),
    range2({ label: "Posición de la máquina", bounds: LATERAL, value: t.machineZ, format: meters, ends: SIDES, onInput: (r) => set({ machineZ: r }) }),
    range2({ label: "Profundidad de la máquina", bounds: MACHINE_DEPTH, value: t.machineX, format: meters, ends: DEPTHS, onInput: (r) => set({ machineX: r }) }),
    range2({ label: "Dirección del tiro", bounds: LATERAL, value: t.feedZ, format: meters, ends: SIDES, onInput: (r) => set({ feedZ: r }) }),
    range2({ label: "Profundidad del tiro", bounds: FEED_DEPTH, value: t.feedX, format: meters, ends: DEPTHS, onInput: (r) => set({ feedX: r }) }),
  );
  pista.querySelector(".fx-tmap")?.append(map.root);

  const golpe = card("Golpe",
    chips("Golpes", SHOT_CHIPS, t.shots, (m) => set({ shots: m })),
    range2({ label: "Potencia", bounds: PERCENT, value: t.power, format: percent, onInput: (r) => set({ power: r }) }),
    range2({ label: "Calidad del golpe", bounds: PERCENT, value: t.quality, format: percent, onInput: (r) => set({ quality: r }) }),
    el("p", "fx-note", RANDOM_HINT),
  );

  interval.append(slider({
    label: "Intervalo", min: INTERVAL.min, max: INTERVAL.max, step: INTERVAL.step, value: t.interval,
    format: seconds, onInput: (v) => set({ interval: v }),
  }));
  const sesion = card("Sesión",
    segmented("Alimentación", FEEDING, t.onDemand, (v) => set({ onDemand: v })),
    interval,
    segmented("Bote", BOUNCE, t.volleyOnly, (v) => set({ volleyOnly: v })),
    segmented("Devolución válida", VALID, t.validBounce, (v) => set({ validBounce: v })),
  );
  set({}); // initial paint of minimap + interval visibility

  const root = el("section", "fx-card fx-train");
  applyThemeCss(root); // --train-* minimap tokens from env/theme.ts
  const grid = el("div", "fx-train__grid");
  grid.append(pista, golpe, sesion);
  const actions = el("div", "fx-actions");
  actions.append(
    button("Volver", onBack, "fx-btn fx-btn--ghost"),
    button("Empezar", () => onStart(t), "fx-btn fx-btn--hero"),
  );
  root.append(
    el("h2", "fx-card__title", "Entrenamiento"),
    el("p", "fx-train__sub", "Ajustá la máquina y empezá la sesión."),
    grid, actions,
  );
  return root;
}
