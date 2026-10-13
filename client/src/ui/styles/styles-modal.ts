import { CharacterStyle, styleFromSeed } from "../../character/character";
import { BUILD_RANGE, HAIR_STYLES, HEADWEAR, HEAD_SIZE_RANGE, HairStyle, Headwear, MAX_SHIRT_NUMBER } from "../../character/style";
import { accessoryRows } from "./accessory-rows";
import { swatches } from "./swatches";
import { THEME } from "../../env/theme";
import { t } from "../../i18n/t";
import { loadStyle, saveStyle } from "../../profile/style-store";
import { Choice, button, el, segmented, slider } from "../dom";
import { registerOverlay } from "../overlays";
import { StylePreview } from "./style-preview";

// "Estilos" (lobby dock): customize your own character — hair, headwear,
// skin/hair/hat colors, head size, build and shirt number — with a live 3D preview. Every
// change is saved at once (style-store) and reaches your player in matches.

const HAIR_LABELS: Readonly<Record<HairStyle, string>> = {
  short: t("Corto", "Short"), buzz: t("Rapado", "Buzz"), mohawk: t("Mohicano", "Mohawk"),
  afro: "Afro", bald: t("Pelado", "Bald"), ponytail: t("Colita", "Ponytail"),
};
const HEADWEAR_LABELS: Readonly<Record<Headwear, string>> = {
  none: t("Nada", "None"), cap: t("Gorra", "Cap"), capBack: t("Gorra atrás", "Backwards cap"),
  headband: t("Vincha", "Headband"), beanie: t("Gorro de lana", "Beanie"),
};
const STEP = 0.01;
const SAVE_DELAY_MS = 250; // a slider drag saves once it settles, not per input event
const pct = (v: number): string => `${Math.round(v * 100)}%`;

export class StylesModal {
  private readonly root = el("div", "fx-styles", "", { role: "dialog", "aria-modal": "true", "aria-label": t("Estilos", "Styles") });
  private readonly form = el("div", "fx-styles__form");
  private readonly preview = new StylePreview();
  private readonly openCbs: Array<() => void> = [];
  private style: CharacterStyle = { ...loadStyle() };
  private readonly closeBtn = button("✕", () => this.close(), "fx-styles__close");
  private returnFocus: HTMLElement | null = null;
  private saveTimer = 0;

  constructor(parent: HTMLElement) {
    const close = this.closeBtn;
    close.setAttribute("aria-label", t("Cerrar", "Close"));
    const stage = el("div", "fx-styles__stage");
    stage.append(this.preview.canvas);
    const body = el("div", "fx-styles__body");
    body.append(stage, this.form);
    const random = button(t("Aleatorio", "Random"), () => this.set(styleFromSeed(Date.now() & 0x7fffffff), true, true), "fx-btn fx-btn--ghost");
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
    this.returnFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    this.set({ ...loadStyle() }, true, false); // show the saved style: nothing to save yet
    this.root.hidden = false;
    this.preview.start();
    this.closeBtn.focus({ preventScroll: true });
    for (const cb of this.openCbs) cb();
  }

  close(): void {
    if (!this.isOpen) return;
    this.root.hidden = true;
    this.preview.stop();
    this.flushSave();
    for (const cb of this.openCbs) cb();
    this.returnFocus?.focus({ preventScroll: true });
  }

  private flushSave(): void {
    if (!this.saveTimer) return;
    window.clearTimeout(this.saveTimer);
    this.saveTimer = 0;
    saveStyle(this.style);
  }

  /** Applies a style to the preview, saves it (debounced) and, on rebuild, redraws the controls. */
  private set(style: CharacterStyle, rebuild: boolean, save: boolean): void {
    this.style = style;
    this.preview.setStyle(style);
    if (rebuild) this.build();
    if (!save) return;
    window.clearTimeout(this.saveTimer);
    this.saveTimer = window.setTimeout(() => this.flushSave(), SAVE_DELAY_MS);
  }

  private patch(p: Partial<CharacterStyle>): void {
    this.set({ ...this.style, ...p }, false, true);
  }

  private build(): void {
    const s = this.style;
    const fx = THEME.fx;
    const hair: Choice<HairStyle>[] = HAIR_STYLES.map((v) => ({ value: v, label: HAIR_LABELS[v] }));
    const wear: Choice<Headwear>[] = HEADWEAR.map((v) => ({ value: v, label: HEADWEAR_LABELS[v] }));
    this.form.replaceChildren(
      segmented(t("Pelo", "Hair"), hair, s.hair, (v) => this.patch({ hair: v })),
      swatches(t("Color de pelo", "Hair color"), fx.hairColors, s.hairColor, (i) => this.patch({ hairColor: i })),
      segmented(t("Cabeza", "Headwear"), wear, s.headwear, (v) => this.patch({ headwear: v })),
      swatches(t("Color de gorro", "Headwear color"), fx.hatColors, s.hatColor, (i) => this.patch({ hatColor: i })),
      swatches(t("Piel", "Skin"), fx.skinTones, s.skin, (i) => this.patch({ skin: i })),
      ...accessoryRows(s, (p) => this.patch(p)),
      slider({ label: t("Tamaño de cabeza", "Head size"), min: HEAD_SIZE_RANGE[0], max: HEAD_SIZE_RANGE[1], step: STEP,
        value: s.headSize, format: pct, onInput: (v) => this.patch({ headSize: v }) }),
      slider({ label: t("Contextura", "Build"), min: BUILD_RANGE[0], max: BUILD_RANGE[1], step: STEP,
        value: s.build, format: pct, onInput: (v) => this.patch({ build: v }) }),
      slider({ label: t("Número de camiseta", "Shirt number"), min: 0, max: MAX_SHIRT_NUMBER, step: 1,
        value: s.number, format: (v) => String(Math.round(v)), onInput: (v) => this.patch({ number: Math.round(v) }) }),
    );
  }
}
