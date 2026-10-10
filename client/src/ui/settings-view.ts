/** "Ajustes" screen: tabbed panels that emit immutable settings updates. */
import { Choice, el, segmented, slider, toggle } from "./dom";
import { keybindView } from "./keybind-view";
import { t } from "../i18n/t";
import type { Lang } from "../i18n/lang";
import {
  FpsCap, HUD_SCALE_MAX, LandingMarker, HUD_SCALE_MIN, Quality, SENS_MAX, SENS_MIN, Settings, VOLUME_MAX, VOLUME_MIN,
} from "./settings";

export type SettingsUpdate = (fn: (s: Settings) => Settings) => void;

type TabId = "controls" | "video" | "audio" | "game";
const TABS: readonly Choice<TabId>[] = [
  { value: "controls", label: t("Controles", "Controls") },
  { value: "video", label: t("Video", "Video") },
  { value: "audio", label: t("Audio", "Audio") },
  { value: "game", label: t("Juego", "Game") },
];

const QUALITY_CHOICES: readonly Choice<Quality>[] = [
  { value: "low", label: t("Baja", "Low") }, { value: "med", label: t("Media", "Medium") }, { value: "high", label: t("Alta", "High") },
];
const FPS_CHOICES: readonly Choice<FpsCap>[] = [
  { value: 60, label: "60" }, { value: 144, label: "144" }, { value: 240, label: "240" }, { value: 0, label: t("Sin límite", "No limit") },
];

const LANDING_CHOICES: readonly Choice<LandingMarker>[] = [
  { value: "always", label: t("Siempre", "Always") }, { value: "easy", label: t("Solo Fácil", "Easy only") }, { value: "never", label: t("Nunca", "Never") },
];

const LANG_CHOICES: readonly Choice<Lang>[] = [{ value: "es", label: "Español" }, { value: "en", label: "English" }];
const SENS_STEP = 0.05;
const HUD_STEP = 0.05;
const PERCENT = 100;
const fmtX = (v: number): string => `${v.toFixed(2)}×`;
const fmtPct = (v: number): string => `${Math.round(v)}%`;

function controlsPanel(s: Settings, up: SettingsUpdate): HTMLElement[] {
  return [
    slider({ label: t("Sensibilidad del mouse", "Mouse sensitivity"), min: SENS_MIN, max: SENS_MAX, step: SENS_STEP,
      value: s.controls.mouseSensitivity, format: fmtX,
      onInput: (v) => up((p) => ({ ...p, controls: { ...p.controls, mouseSensitivity: v } })) }),
    ...keybindView(s.controls.keys, (keys) => up((p) => ({ ...p, controls: { ...p.controls, keys } }))),
  ];
}

function videoPanel(s: Settings, up: SettingsUpdate): HTMLElement[] {
  const set = (patch: Partial<Settings["video"]>): void => up((p) => ({ ...p, video: { ...p.video, ...patch } }));
  return [
    segmented(t("Calidad", "Quality"), QUALITY_CHOICES, s.video.quality, (quality) => set({ quality })),
    toggle(t("Sombras", "Shadows"), s.video.shadows, (shadows) => set({ shadows })),
    segmented(t("Límite de FPS", "FPS limit"), FPS_CHOICES, s.video.fpsCap, (fpsCap) => set({ fpsCap })),
    toggle(t("Mostrar FPS", "Show FPS"), s.video.showFps, (showFps) => set({ showFps })),
  ];
}

function audioPanel(s: Settings, up: SettingsUpdate): HTMLElement[] {
  const vol = (label: string, key: keyof Settings["audio"]): HTMLElement => slider({
    label, min: VOLUME_MIN, max: VOLUME_MAX, step: 1, value: s.audio[key], format: fmtPct,
    onInput: (v) => up((p) => ({ ...p, audio: { ...p.audio, [key]: v } })),
  });
  return [vol(t("General", "Master"), "master"), vol(t("Efectos", "Effects"), "sfx"), vol(t("Música", "Music"), "music"),
    el("p", "fx-note", t("Todavía no hay música: el volumen se guarda para cuando llegue.", "No music yet: the volume is saved for when it arrives."))];
}

function gamePanel(s: Settings, up: SettingsUpdate): HTMLElement[] {
  const set = (patch: Partial<Settings["game"]>): void => up((p) => ({ ...p, game: { ...p.game, ...patch } }));
  return [
    toggle(t("Círculo de desvío de puntería", "Aim deflection circle"), s.game.showAimCircle, (showAimCircle) => set({ showAimCircle })),
    segmented(t("Marcador de pique", "Bounce marker"), LANDING_CHOICES, s.game.landingMarker, (landingMarker) => set({ landingMarker })),
    toggle(t("Cámara dinámica (off = fija)", "Dynamic camera (off = fixed)"), s.game.dynamicCamera, (dynamicCamera) => set({ dynamicCamera })),
    toggle(t("Vibración de cámara", "Camera shake"), s.game.cameraShake, (cameraShake) => set({ cameraShake })),
    slider({ label: t("Escala del HUD", "HUD scale"), min: HUD_SCALE_MIN, max: HUD_SCALE_MAX, step: HUD_STEP,
      value: s.game.hudScale, format: (v) => fmtPct(v * PERCENT), onInput: (hudScale) => set({ hudScale }) }),
    toggle(t("Modo daltónico", "Colorblind mode"), s.game.colorblind, (colorblind) => set({ colorblind })),
    segmented("Idioma / Language", LANG_CHOICES, s.game.lang, (lang) => set({ lang })),
    el("p", "fx-note", t("Modo daltónico: calidad del golpe, equipos y avisos con colores distinguibles. Idioma y modo daltónico se aplican al recargar la página.", "Colorblind mode: shot quality, teams and alerts in distinguishable colors. Language and colorblind mode apply after reloading the page.")),
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
