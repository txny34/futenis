// UI language (backlog "Idiomas: español e inglés"). Read once at load from
// the saved settings, like the colorblind palette: module-level strings are
// built at import time, so switching applies on reload.
export type Lang = "es" | "en";
export const LANGS: readonly Lang[] = ["es", "en"];

/** First visit: Spanish for Spanish browsers, English otherwise. */
export function defaultLang(): Lang {
  try { return navigator.language.toLowerCase().startsWith("es") ? "es" : "en"; } catch { return "es"; }
}
