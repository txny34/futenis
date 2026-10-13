import { CharacterStyle } from "../../character/character";
import { EYEWEAR, Eyewear, FACIAL_HAIR, FacialHair } from "../../character/style";
import { THEME } from "../../env/theme";
import { t } from "../../i18n/t";
import { Choice, segmented } from "../dom";
import { swatches } from "./swatches";

// Estilos rows for the accessories: glasses (+ frame color), facial hair,
// wristbands (+ color) and sneaker color. Every accessory is editable.

const EYEWEAR_LABELS: Readonly<Record<Eyewear, string>> = {
  none: t("Nada", "None"), sport: t("Deportivos", "Sport"), round: t("Redondos", "Round"), shades: t("De sol", "Shades"),
};
const FACIAL_LABELS: Readonly<Record<FacialHair, string>> = {
  none: t("Nada", "None"), stubble: t("Candado corto", "Stubble"), mustache: t("Bigote", "Mustache"), beard: t("Barba", "Beard"),
};
const WRIST: Choice<"off" | "on">[] = [{ value: "off", label: t("Sin", "Off") }, { value: "on", label: t("Con", "On") }];

type Patch = (p: Partial<CharacterStyle>) => void;

export function accessoryRows(s: Readonly<CharacterStyle>, patch: Patch): HTMLElement[] {
  const fx = THEME.fx;
  const eyewear: Choice<Eyewear>[] = EYEWEAR.map((v) => ({ value: v, label: EYEWEAR_LABELS[v] }));
  const facial: Choice<FacialHair>[] = FACIAL_HAIR.map((v) => ({ value: v, label: FACIAL_LABELS[v] }));
  return [
    segmented(t("Anteojos", "Glasses"), eyewear, s.eyewear, (v) => patch({ eyewear: v })),
    swatches(t("Color de anteojos", "Glasses color"), fx.eyewearColors, s.eyewearColor, (i) => patch({ eyewearColor: i })),
    segmented(t("Barba", "Facial hair"), facial, s.facialHair, (v) => patch({ facialHair: v })),
    segmented(t("Muñequeras", "Wristbands"), WRIST, s.wristbands ? "on" : "off", (v) => patch({ wristbands: v === "on" })),
    swatches(t("Color de muñequeras", "Wristband color"), fx.gearColors, s.wristColor, (i) => patch({ wristColor: i })),
    swatches(t("Zapatillas", "Sneakers"), fx.bootColors, s.bootsColor, (i) => patch({ bootsColor: i })),
  ];
}
