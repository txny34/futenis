import { CharacterStyle, styleFromSeed } from "../../character/character";
import { BUILD_RANGE, HAIR_STYLES, HEADWEAR, HEAD_SIZE_RANGE, HairStyle, Headwear } from "../../character/style";
import { THEME } from "../../env/theme";
import { t } from "../../i18n/t";
import { loadStyle, saveStyle } from "../../profile/style-store";
import { Choice, button, el, segmented, slider } from "../dom";
import { registerOverlay } from "../overlays";
import { StylePreview } from "./style-preview";

// "Estilos" (lobby dock): customize your own character — hair, headwear,
// skin/hair/hat colors, head size and build — with a live 3D preview. Every
// change is saved at once (style-store) and reaches your player in matches.

const HAIR_LABELS: Readonly<Record<HairStyle, string>> = {
  short: t("Corto", "Short"), buzz: t("Rapado", "Buzz"), mohawk: t("Mohicano", "Mohawk"),
  afro: "Afro", bald: t("Pelado", "Bald"), ponytail: t("Colita", "Ponytail"),
};
const HEADWEAR_LABELS: Readonly<Record<Headwear, string>> = {
  none: t("Nada", "None"), cap: t("Gorra", "Cap"), headband: t("Vincha", "Headband"),
};
const STEP = 0.01;
const pct = (v: number): string => `${Math.round(v * 100)}%`;
const hex = (c: number): string => `#${c.toString(16).padStart(6, "0")}`;

function swatches(label: string, colors: readonly number[], value: number, onPick: (i: number) => void): HTMLElement {
  const row = el("div", "fx-row");
  const group = el("div", "fx-styles__swatches", "", { role: "radiogroup", "aria-label": label });
  colors.forEach((c, i) => {
    const b = el("button", "fx-styles__swatch", "", { type: "button", role: "radio", "aria-checked": String(i === value), "aria-label": `${label} ${i + 1}` });
    b.style.background = hex(c);
    b.addEventListener("click", () => onPick(i));
    group.append(b);
  });
  row.append(el("span", "fx-row__label", label), group);
  return row;
}

export class StylesModal {
  private readonly root = el("div", "fx-styles", "", { role: "dialog", "aria-modal": "true", "aria-label": t("Estilos", "Styles") });
  private readonly form = el("div", "fx-styles__form");
  private readonly preview = new StylePreview();
  private readonly openCbs: Array<() => void> = [];
  private style: CharacterStyle = { ...loadStyle() };

  constructor(parent: HTMLElement) {
    const close = button("✕", () => this.close(), "fx-styles__close");
    close.setAttribute("aria-label", t("Cerrar", "Close"));
    const stage = el("div", "fx-styles__stage");
    stage.append(this.preview.canvas);
    const body = el("div", "fx-styles__body");
    body.append(stage, this.form);
    const random = button(t("Aleatorio", "Random"), () => this.set(styleFromSeed(Date.now() & 0x7fffffff), true), "fx-btn fx-btn--ghost");
    const done = button(t("Listo", "Done"), () => this.close(), "fx-btn fx-btn--hero");
    const foot = el("div", "fx-styles__foot");
    foot.append(random, done);
    this.root.append(close, el("h2", "fx-styles__title", t("Estilos", "Styles")), body, foot);
    this.root.hidden = true;
    parent.append(this.root);
    registerOverlay(() => this.isOpen);
    window.addEventListener("keydown", (e) => {
      if (this.isOpen && e.key === "Escape") { e.preventDefault(); e.stopImmediatePropagation(); this.close(); }
    }, true);
  }

  get isOpen(): boolean { return !this.root.hidden; }
  onOpenChange(cb: () => void): void { this.openCbs.push(cb); }

  open(): void {
    if (this.isOpen) return;
    this.set({ ...loadStyle() }, true);
    this.root.hidden = false;
    this.preview.start();
    for (const cb of this.openCbs) cb();
  }

  close(): void {
    if (!this.isOpen) return;
    this.root.hidden = true;
    this.preview.stop();
    for (const cb of this.openCbs) cb();
  }

  /** Applies and saves a style; rebuild redraws the controls (Random, open). */
  private set(style: CharacterStyle, rebuild = false): void {
    this.style = style;
    saveStyle(style);
    this.preview.setStyle(style);
    if (rebuild) this.build();
  }

  private patch(p: Partial<CharacterStyle>, rebuild = false): void {
    this.set({ ...this.style, ...p }, rebuild);
  }

  private build(): void {
    const s = this.style;
    const fx = THEME.fx;
    const hair: Choice<HairStyle>[] = HAIR_STYLES.map((v) => ({ value: v, label: HAIR_LABELS[v] }));
    const wear: Choice<Headwear>[] = HEADWEAR.map((v) => ({ value: v, label: HEADWEAR_LABELS[v] }));
    this.form.replaceChildren(
      segmented(t("Pelo", "Hair"), hair, s.hair, (v) => this.patch({ hair: v })),
      swatches(t("Color de pelo", "Hair color"), fx.hairColors, s.hairColor, (i) => this.patch({ hairColor: i }, true)),
      segmented(t("Cabeza", "Headwear"), wear, s.headwear, (v) => this.patch({ headwear: v })),
      swatches(t("Color de gorro", "Headwear color"), fx.hatColors, s.hatColor, (i) => this.patch({ hatColor: i }, true)),
      swatches(t("Piel", "Skin"), fx.skinTones, s.skin, (i) => this.patch({ skin: i }, true)),
      slider({ label: t("Tamaño de cabeza", "Head size"), min: HEAD_SIZE_RANGE[0], max: HEAD_SIZE_RANGE[1], step: STEP,
        value: s.headSize, format: pct, onInput: (v) => this.patch({ headSize: v }) }),
      slider({ label: t("Contextura", "Build"), min: BUILD_RANGE[0], max: BUILD_RANGE[1], step: STEP,
        value: s.build, format: pct, onInput: (v) => this.patch({ build: v }) }),
    );
  }
}
