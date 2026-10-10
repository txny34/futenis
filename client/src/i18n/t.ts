import { loadSettings } from "../ui/settings";
import type { Lang } from "./lang";

export const LANG: Lang = loadSettings().game.lang;

/** Inline pair: t("Reanudar", "Resume"). Spanish stays the source text next to the code. */
export function t(es: string, en: string): string {
  return LANG === "en" ? en : es;
}
