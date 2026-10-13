import { el } from "../dom";

// A labeled row of color swatches (Estilos): picks one palette index.

const hex = (c: number): string => `#${c.toString(16).padStart(6, "0")}`;

export function swatches(label: string, colors: readonly number[], value: number, onPick: (i: number) => void): HTMLElement {
  const row = el("div", "fx-row");
  const group = el("div", "fx-styles__swatches", "", { role: "radiogroup", "aria-label": label });
  colors.forEach((c, i) => {
    const b = el("button", "fx-styles__swatch", "", { type: "button", role: "radio", "aria-checked": String(i === value), "aria-label": `${label} ${i + 1}` });
    b.style.background = hex(c);
    b.addEventListener("click", () => {
      for (const o of group.children) o.setAttribute("aria-checked", String(o === b)); // in place: focus stays
      onPick(i);
    });
    group.append(b);
  });
  row.append(el("span", "fx-row__label", label), group);
  return row;
}
