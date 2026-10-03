/** Tiny DOM builders for the menu. No framework, no innerHTML with data. */

type Attrs = Record<string, string>;

export function el<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  cls = "",
  text = "",
  attrs: Attrs = {},
): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text) e.textContent = text;
  for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, v);
  return e;
}

export function button(label: string, onClick: () => void, cls = "fx-btn"): HTMLButtonElement {
  const b = el("button", cls, label, { type: "button" });
  b.addEventListener("click", onClick);
  return b;
}

export function disabledButton(label: string, badge: string): HTMLButtonElement {
  const b = el("button", "fx-btn fx-btn--soon", label, { type: "button", "aria-disabled": "true" });
  b.disabled = true;
  b.append(el("span", "fx-badge", badge));
  return b;
}

let idSeq = 0;
function nextId(): string {
  idSeq += 1;
  return `fx-ctl-${idSeq}`;
}

function row(label: string, id: string, control: HTMLElement): HTMLElement {
  const r = el("div", "fx-row");
  r.append(el("label", "fx-row__label", label, { for: id }), control);
  return r;
}

export interface SliderSpec {
  label: string; min: number; max: number; step: number; value: number;
  format: (v: number) => string;
  onInput: (v: number) => void;
}

export function slider(s: SliderSpec): HTMLElement {
  const id = nextId();
  const wrap = el("div", "fx-slider");
  const input = el("input", "", "", {
    id, type: "range", min: String(s.min), max: String(s.max), step: String(s.step),
  });
  input.value = String(s.value);
  const out = el("output", "fx-slider__value", s.format(s.value), { for: id });
  input.addEventListener("input", () => {
    const v = Number(input.value);
    out.textContent = s.format(v);
    s.onInput(v);
  });
  wrap.append(input, out);
  return row(s.label, id, wrap);
}

export function toggle(label: string, value: boolean, onChange: (v: boolean) => void): HTMLElement {
  const id = nextId();
  const input = el("input", "fx-toggle", "", { id, type: "checkbox", role: "switch" });
  input.checked = value;
  input.addEventListener("change", () => onChange(input.checked));
  return row(label, id, input);
}

export interface Choice<T> { value: T; label: string }

/** Segmented single-choice control (radio group styled as pills). */
export function segmented<T>(label: string, choices: readonly Choice<T>[], value: T, onPick: (v: T) => void): HTMLElement {
  const group = el("div", "fx-seg", "", { role: "radiogroup", "aria-label": label });
  const name = nextId();
  for (const c of choices) {
    const id = nextId();
    const input = el("input", "fx-seg__input", "", { id, type: "radio", name });
    input.checked = c.value === value;
    input.addEventListener("change", () => { if (input.checked) onPick(c.value); });
    group.append(input, el("label", "fx-seg__pill", c.label, { for: id }));
  }
  const r = el("div", "fx-row");
  r.append(el("span", "fx-row__label", label), group);
  return r;
}
