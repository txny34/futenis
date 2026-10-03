/** Double-ended range slider: two overlaid native range inputs (keyboard + a11y for free) on one track. */
import { el } from "./dom";
import type { Bounds, Range } from "./training-settings";

export interface Range2Spec {
  readonly label: string;
  readonly bounds: Bounds;
  readonly value: Range;
  readonly format: (v: number) => string;
  /** Optional end captions under the track (e.g. "Izquierda" / "Derecha"). */
  readonly ends?: readonly [string, string];
  readonly onInput: (r: Range) => void;
}

function thumb(b: Bounds, v: number, label: string): HTMLInputElement {
  const input = el("input", "fx-range2__input", "", {
    type: "range", min: String(b.min), max: String(b.max), step: String(b.step), "aria-label": label,
  });
  input.value = String(v);
  return input;
}

/** Builds the control. Thumbs never cross: the moved one is clamped to the other. */
export function range2(s: Range2Spec): HTMLElement {
  const b = s.bounds;
  const wrap = el("div", "fx-range2");
  const head = el("div", "fx-range2__head");
  const out = el("output", "fx-range2__value");
  head.append(el("span", "fx-row__label", s.label), out);
  const track = el("div", "fx-range2__track");
  const fill = el("div", "fx-range2__fill");
  const lo = thumb(b, s.value.lo, `${s.label} mínimo`);
  const hi = thumb(b, s.value.hi, `${s.label} máximo`);
  track.append(fill, lo, hi);
  wrap.append(head, track);
  if (s.ends) {
    const ends = el("div", "fx-range2__ends");
    ends.append(el("span", "", s.ends[0]), el("span", "", s.ends[1]));
    wrap.append(ends);
  }

  const pct = (v: number): number => ((v - b.min) / (b.max - b.min)) * 100;
  const paint = (a: number, z: number): void => {
    fill.style.left = `${pct(a)}%`;
    fill.style.right = `${100 - pct(z)}%`;
    out.textContent = a === z ? s.format(a) : `${s.format(a)} – ${s.format(z)}`;
    // Keep the low thumb grabbable when both sit at the max end.
    lo.style.zIndex = a >= b.max - b.step ? "3" : "1";
  };
  const onMove = (moved: HTMLInputElement): void => {
    let a = Number(lo.value);
    let z = Number(hi.value);
    if (a > z) {
      if (moved === lo) a = z;
      else z = a;
      lo.value = String(a);
      hi.value = String(z);
    }
    paint(a, z);
    s.onInput({ lo: a, hi: z });
  };
  lo.addEventListener("input", () => onMove(lo));
  hi.addEventListener("input", () => onMove(hi));
  paint(s.value.lo, s.value.hi);
  return wrap;
}
