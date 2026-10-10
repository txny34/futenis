/** Entrenamiento setup panel: PISTA / GOLPE / SESIÓN cards + Volver / Empezar. Pure builder. */
import { challengesCard } from "./challenges-card";
import { applyThemeCss } from "../env/theme";
import { button, Choice, el, segmented, slider } from "./dom";
import { range2 } from "./range2";
import { minimap } from "./training-minimap";
import {
  FEED_DEPTH, INTERVAL, LATERAL, loadTraining, MACHINE_DEPTH, PERCENT, saveTraining,
  SHOT_DRIVE, SHOT_LOB, SHOT_SMASH, TrainingSettings,
} from "./training-settings";
// Aliased: the trainingView() function below uses a local `t` for TrainingSettings.
import { t as tr } from "../i18n/t";

type Patch = Partial<TrainingSettings>;

const SIDES: readonly [string, string] = [tr("Izquierda", "Left"), tr("Derecha", "Right")];
const DEPTHS: readonly [string, string] = [tr("Red", "Net"), tr("Fondo", "Back")];
const SHOT_CHIPS: readonly Choice<number>[] = [
  { value: SHOT_DRIVE, label: tr("Derecha", "Drive") }, { value: SHOT_LOB, label: tr("Globo", "Lob") }, { value: SHOT_SMASH, label: tr("Remate", "Smash") },
];
const FEEDING: readonly Choice<boolean>[] = [{ value: false, label: tr("Continua", "Continuous") }, { value: true, label: tr("A demanda", "On demand") }];
const BOUNCE: readonly Choice<boolean>[] = [{ value: false, label: tr("Permitido", "Allowed") }, { value: true, label: tr("Solo volea", "Volley only") }];
const VALID: readonly Choice<1 | 2>[] = [{ value: 1, label: tr("1.er bote", "1st bounce") }, { value: 2, label: tr("2.º bote", "2nd bounce") }];
const RANDOM_HINT = tr("Cada bola toma un valor al azar dentro del rango. Acotalo para repetir la misma bola.", "Each ball picks a random value within the range. Narrow it to repeat the same ball.");

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

const SAVE_DEBOUNCE_MS = 250;

/** onStart receives the final settings (already persisted). */
export function trainingView(onStart: (t: TrainingSettings) => void, onBack: () => void): HTMLElement {
  let t = loadTraining();
  const map = minimap();
  const interval = el("div", "fx-tinterval");
  let saveTimer = 0;
  const flush = (): void => {
    window.clearTimeout(saveTimer);
    saveTraining(t);
  };
  const set = (p: Patch): void => {
    t = { ...t, ...p };
    map.update(t);
    interval.hidden = t.onDemand;
    window.clearTimeout(saveTimer); // sliders fire per drag tick: persist once they settle
    saveTimer = window.setTimeout(flush, SAVE_DEBOUNCE_MS);
  };

  const pista = card(tr("Pista", "Court"),
    el("div", "fx-tmap"),
    range2({ label: tr("Posición de la máquina", "Machine position"), bounds: LATERAL, value: t.machineZ, format: meters, ends: SIDES, onInput: (r) => set({ machineZ: r }) }),
    range2({ label: tr("Profundidad de la máquina", "Machine depth"), bounds: MACHINE_DEPTH, value: t.machineX, format: meters, ends: DEPTHS, onInput: (r) => set({ machineX: r }) }),
    range2({ label: tr("Dirección del tiro", "Shot direction"), bounds: LATERAL, value: t.feedZ, format: meters, ends: SIDES, onInput: (r) => set({ feedZ: r }) }),
    range2({ label: tr("Profundidad del tiro", "Shot depth"), bounds: FEED_DEPTH, value: t.feedX, format: meters, ends: DEPTHS, onInput: (r) => set({ feedX: r }) }),
  );
  pista.querySelector(".fx-tmap")?.append(map.root);

  const golpe = card(tr("Golpe", "Shot"),
    chips(tr("Golpes", "Shots"), SHOT_CHIPS, t.shots, (m) => set({ shots: m })),
    range2({ label: tr("Potencia", "Power"), bounds: PERCENT, value: t.power, format: percent, onInput: (r) => set({ power: r }) }),
    range2({ label: tr("Calidad del golpe", "Shot quality"), bounds: PERCENT, value: t.quality, format: percent, onInput: (r) => set({ quality: r }) }),
    el("p", "fx-note", RANDOM_HINT),
  );

  interval.append(slider({
    label: tr("Intervalo", "Interval"), min: INTERVAL.min, max: INTERVAL.max, step: INTERVAL.step, value: t.interval,
    format: seconds, onInput: (v) => set({ interval: v }),
  }));
  const sesion = card(tr("Sesión", "Session"),
    segmented(tr("Alimentación", "Feeding"), FEEDING, t.onDemand, (v) => set({ onDemand: v })),
    interval,
    segmented(tr("Bote", "Bounce"), BOUNCE, t.volleyOnly, (v) => set({ volleyOnly: v })),
    segmented(tr("Devolución válida", "Valid return"), VALID, t.validBounce, (v) => set({ validBounce: v })),
  );
  set({}); // initial paint of minimap + interval visibility

  const root = el("section", "fx-card fx-train");
  applyThemeCss(root); // --train-* minimap tokens from env/theme.ts
  const grid = el("div", "fx-train__grid");
  grid.append(pista, golpe, sesion);
  const actions = el("div", "fx-actions");
  actions.append(
    button(tr("Volver", "Back"), () => { flush(); onBack(); }, "fx-btn fx-btn--ghost"),
    button(tr("Empezar", "Start"), () => { flush(); onStart(t); }, "fx-btn fx-btn--hero"),
  );
  root.append(
    el("h2", "fx-card__title", tr("Entrenamiento", "Training")),
    el("p", "fx-train__sub", tr("Ajustá la máquina y empezá la sesión.", "Set up the machine and start the session.")),
    grid, challengesCard(), actions,
  );
  return root;
}
