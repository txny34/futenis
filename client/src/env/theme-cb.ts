import { loadSettings } from "../ui/settings";

// Colorblind-safe palette (Okabe–Ito) for everything that encodes meaning by
// hue: timing quality, impacts, teams and warnings. Read once from the saved
// settings at load: THEME is built at import time, so the toggle applies on reload.

export const COLORBLIND = loadSettings().game.colorblind;

export const CB = {
  yellow: 0xf0e442, sky: 0x56b4e9, vermillion: 0xd55e00, purple: 0xcc79a7,
  yellowCss: "#f0e442", skyCss: "#56b4e9", vermillionCss: "#d55e00", purpleCss: "#cc79a7",
} as const;
