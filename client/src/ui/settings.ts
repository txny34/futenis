/** Player settings: types, defaults, validation and localStorage persistence. */
import { defaultLang, type Lang, LANGS } from "../i18n/lang";
import { DEFAULT_KEYS, type KeyBinds, validateKeys } from "./keybinds";

export type Quality = "low" | "med" | "high";
export type FpsCap = 60 | 144 | 240 | 0; // 0 = sin límite

export interface ControlSettings { mouseSensitivity: number; keys: KeyBinds }
export interface VideoSettings { quality: Quality; shadows: boolean; fpsCap: FpsCap; showFps: boolean }
export interface AudioSettings { master: number; sfx: number; music: number }
/** "Marcador de pique": siempre / solo en Fácil / nunca. */
export type LandingMarker = "always" | "easy" | "never";
export interface GameplaySettings {
  showAimCircle: boolean; cameraShake: boolean; dynamicCamera: boolean; hudScale: number; landingMarker: LandingMarker;
  colorblind: boolean; lang: Lang;
}

export interface Settings {
  readonly controls: ControlSettings;
  readonly video: VideoSettings;
  readonly audio: AudioSettings;
  readonly game: GameplaySettings;
}

export const SETTINGS_SCHEMA_VERSION = 1;
const STORAGE_KEY = "futenis.settings";

export const SENS_MIN = 0.1;
export const SENS_MAX = 3;
export const VOLUME_MIN = 0;
export const VOLUME_MAX = 100;
export const HUD_SCALE_MIN = 0.75;
export const HUD_SCALE_MAX = 1.5;

export const QUALITIES: readonly Quality[] = ["low", "med", "high"];
export const FPS_CAPS: readonly FpsCap[] = [60, 144, 240, 0];
export const LANDING_MARKERS: readonly LandingMarker[] = ["always", "easy", "never"];

export const DEFAULT_SETTINGS: Settings = {
  controls: { mouseSensitivity: 1, keys: DEFAULT_KEYS },
  video: { quality: "high", shadows: true, fpsCap: 0, showFps: false },
  audio: { master: 80, sfx: 80, music: 60 },
  game: { showAimCircle: true, cameraShake: true, dynamicCamera: true, hudScale: 1, landingMarker: "easy", colorblind: false, lang: defaultLang() },
};

type Rec = Record<string, unknown>;

function isRec(v: unknown): v is Rec {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

function num(v: unknown, min: number, max: number, def: number): number {
  return typeof v === "number" && Number.isFinite(v) && v >= min && v <= max ? v : def;
}

function bool(v: unknown, def: boolean): boolean {
  return typeof v === "boolean" ? v : def;
}

function oneOf<T>(v: unknown, opts: readonly T[], def: T): T {
  const hit = opts.find((o) => o === v);
  return hit === undefined ? def : hit;
}

function section(raw: Rec, key: string): Rec {
  const v = raw[key];
  return isRec(v) ? v : {};
}

/** Field-by-field validation: invalid fields fall back to their default. */
export function validateSettings(raw: unknown): Settings {
  const d = DEFAULT_SETTINGS;
  if (!isRec(raw)) return d;
  const c = section(raw, "controls");
  const v = section(raw, "video");
  const a = section(raw, "audio");
  const g = section(raw, "game");
  return {
    controls: {
      mouseSensitivity: num(c.mouseSensitivity, SENS_MIN, SENS_MAX, d.controls.mouseSensitivity),
      keys: validateKeys(c.keys),
    },
    video: {
      quality: oneOf(v.quality, QUALITIES, d.video.quality),
      shadows: bool(v.shadows, d.video.shadows),
      fpsCap: oneOf(v.fpsCap, FPS_CAPS, d.video.fpsCap),
      showFps: bool(v.showFps, d.video.showFps),
    },
    audio: {
      master: num(a.master, VOLUME_MIN, VOLUME_MAX, d.audio.master),
      sfx: num(a.sfx, VOLUME_MIN, VOLUME_MAX, d.audio.sfx),
      music: num(a.music, VOLUME_MIN, VOLUME_MAX, d.audio.music),
    },
    game: {
      showAimCircle: bool(g.showAimCircle, d.game.showAimCircle),
      cameraShake: bool(g.cameraShake, d.game.cameraShake),
      dynamicCamera: bool(g.dynamicCamera, d.game.dynamicCamera),
      hudScale: num(g.hudScale, HUD_SCALE_MIN, HUD_SCALE_MAX, d.game.hudScale),
      landingMarker: oneOf(g.landingMarker, LANDING_MARKERS, d.game.landingMarker),
      colorblind: bool(g.colorblind, d.game.colorblind),
      lang: oneOf(g.lang, LANGS, d.game.lang),
    },
  };
}

/** Loads settings; any storage/parse/version problem yields defaults. */
export function loadSettings(): Settings {
  try {
    const text = window.localStorage.getItem(STORAGE_KEY);
    if (text === null) return DEFAULT_SETTINGS;
    const parsed: unknown = JSON.parse(text);
    if (!isRec(parsed) || parsed.version !== SETTINGS_SCHEMA_VERSION) return DEFAULT_SETTINGS;
    return validateSettings(parsed.data);
  } catch {
    return DEFAULT_SETTINGS;
  }
}

/** Persists settings. Returns false if storage is unavailable (private mode, quota). */
export function saveSettings(s: Settings): boolean {
  try {
    const payload = { version: SETTINGS_SCHEMA_VERSION, data: s };
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(payload));
    return true;
  } catch {
    return false;
  }
}
