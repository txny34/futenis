/** "Ajustes" screen: tabbed panels that emit immutable settings updates. */
import { Choice, el, segmented, slider, toggle } from "./dom";
import {
  FpsCap, HUD_SCALE_MAX, HUD_SCALE_MIN, Quality, SENS_MAX, SENS_MIN, Settings, VOLUME_MAX, VOLUME_MIN,
} from "./settings";

export type SettingsUpdate = (fn: (s: Settings) => Settings) => void;

type TabId = "controls" | "video" | "audio" | "game";
const TABS: readonly Choice<TabId>[] = [
  { value: "controls", label: "Controles" },
  { value: "video", label: "Video" },
  { value: "audio", label: "Audio" },
  { value: "game", label: "Juego" },
];

const BINDINGS: readonly (readonly [string, string])[] = [
  ["Mover", "W A S D"],
  ["Apuntar", "Mouse"],
  ["Cargar / patear", "Click izq."],
  ["Globo", "Click der."],
  ["Efecto", "Rueda"],
  ["Salto", "Espacio"],
  ["Sprint", "Shift"],
];

const QUALITY_CHOICES: readonly Choice<Quality>[] = [
  { value: "low", label: "Baja" }, { value: "med", label: "Media" }, { value: "high", label: "Alta" },
];
const FPS_CHOICES: readonly Choice<FpsCap>[] = [
  { value: 60, label: "60" }, { value: 144, label: "144" }, { value: 240, label: "240" }, { value: 0, label: "Sin límite" },
];

const SENS_STEP = 0.05;
const HUD_STEP = 0.05;
const PERCENT = 100;
const fmtX = (v: number): string => `${v.toFixed(2)}×`;
const fmtPct = (v: number): string => `${Math.round(v)}%`;

function controlsPanel(s: Settings, up: SettingsUpdate): HTMLElement[] {
  const list = el("ul", "fx-binds");
  for (const [action, key] of BINDINGS) {
    const li = el("li", "fx-binds__row");
    li.append(el("span", "", action), el("kbd", "fx-kbd", key));
    list.append(li);
  }
  // TODO(ui): rebinding de teclas (click en una fila → "presioná una tecla").
  return [
    slider({ label: "Sensibilidad del mouse", min: SENS_MIN, max: SENS_MAX, step: SENS_STEP,
      value: s.controls.mouseSensitivity, format: fmtX,
      onInput: (v) => up((p) => ({ ...p, controls: { ...p.controls, mouseSensitivity: v } })) }),
    el("h3", "fx-sub", "Controles"), list,
    el("p", "fx-note", "Reasignar teclas: pronto."),
  ];
}

function videoPanel(s: Settings, up: SettingsUpdate): HTMLElement[] {
  const set = (patch: Partial<Settings["video"]>): void => up((p) => ({ ...p, video: { ...p.video, ...patch } }));
  return [
    segmented("Calidad", QUALITY_CHOICES, s.video.quality, (quality) => set({ quality })),
    toggle("Sombras", s.video.shadows, (shadows) => set({ shadows })),
    segmented("Límite de FPS", FPS_CHOICES, s.video.fpsCap, (fpsCap) => set({ fpsCap })),
    toggle("Mostrar FPS", s.video.showFps, (showFps) => set({ showFps })),
  ];
}

function audioPanel(s: Settings, up: SettingsUpdate): HTMLElement[] {
  const vol = (label: string, key: keyof Settings["audio"]): HTMLElement => slider({
    label, min: VOLUME_MIN, max: VOLUME_MAX, step: 1, value: s.audio[key], format: fmtPct,
    onInput: (v) => up((p) => ({ ...p, audio: { ...p.audio, [key]: v } })),
  });
  return [vol("General", "master"), vol("Efectos", "sfx"), vol("Música", "music"),
    el("p", "fx-note", "Todavía no hay sonido — se guarda para cuando llegue.")];
}

function gamePanel(s: Settings, up: SettingsUpdate): HTMLElement[] {
  const set = (patch: Partial<Settings["game"]>): void => up((p) => ({ ...p, game: { ...p.game, ...patch } }));
  return [
    toggle("Círculo de desvío de puntería", s.game.showAimCircle, (showAimCircle) => set({ showAimCircle })),
    toggle("Cámara dinámica (off = fija)", s.game.dynamicCamera, (dynamicCamera) => set({ dynamicCamera })),
    toggle("Vibración de cámara", s.game.cameraShake, (cameraShake) => set({ cameraShake })),
    slider({ label: "Escala del HUD", min: HUD_SCALE_MIN, max: HUD_SCALE_MAX, step: HUD_STEP,
      value: s.game.hudScale, format: (v) => fmtPct(v * PERCENT), onInput: (hudScale) => set({ hudScale }) }),
  ];
}

const PANELS: Record<TabId, (s: Settings, up: SettingsUpdate) => HTMLElement[]> = {
  controls: controlsPanel, video: videoPanel, audio: audioPanel, game: gamePanel,
};

/** Builds the tabbed settings body. `get` reads the latest settings when a tab opens. */
export function settingsView(get: () => Settings, up: SettingsUpdate): HTMLElement {
  const root = el("div", "fx-settings");
  const bar = el("div", "fx-tabs", "", { role: "tablist" });
  const panel = el("div", "fx-panel", "", { role: "tabpanel" });
  const tabs: HTMLButtonElement[] = [];
  const open = (id: TabId): void => {
    for (const t of tabs) t.setAttribute("aria-selected", String(t.dataset.tab === id));
    panel.replaceChildren(...PANELS[id](get(), up));
  };
  for (const t of TABS) {
    const b = el("button", "fx-tab", t.label, { type: "button", role: "tab", "data-tab": t.value });
    b.addEventListener("click", () => open(t.value));
    tabs.push(b);
  }
  bar.append(...tabs);
  root.append(bar, panel);
  open("controls");
  return root;
}
