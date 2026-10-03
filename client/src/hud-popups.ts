// Floating hit text ("¡PERFECTO!" / "¡BIEN!" / "FALLIDO") that rises from the contact
// point and drifts toward the shot direction while fading. Small pool of
// DOM nodes; position/drift go through CSS custom properties.
const POOL = 3;
const LIFE_MS = 900;
const DRIFT_PX = 70;
const ANIM_CLASS = "is-live";

export const POPUP_PERFECT = "¡PERFECTO!";
export const POPUP_GOOD = "¡BIEN!";
export const POPUP_MISS = "FALLIDO";

interface Popup { readonly el: HTMLDivElement; until: number }

export class Popups {
  readonly root = document.createElement("div");
  private readonly pool: Popup[] = [];
  private next = 0;

  constructor() {
    this.root.id = "popups";
    for (let i = 0; i < POOL; i++) {
      const el = document.createElement("div");
      el.className = "popup";
      el.hidden = true;
      this.root.append(el);
      this.pool.push({ el, until: 0 });
    }
  }

  /** x/y: normalized screen point (0..1). dir: -1 left, 0 straight, +1 right. tone: "perfect" | "good" | "miss". */
  spawn(text: string, tone: string, x: number, y: number, dir: number, nowMs: number): void {
    const p = this.pool[this.next];
    this.next = (this.next + 1) % POOL;
    p.until = nowMs + LIFE_MS;
    const s = p.el.style;
    s.setProperty("--x", `${(Math.max(0, Math.min(1, x)) * 100).toFixed(1)}%`);
    s.setProperty("--y", `${(Math.max(0, Math.min(1, y)) * 100).toFixed(1)}%`);
    s.setProperty("--dx", `${(Math.max(-1, Math.min(1, dir)) * DRIFT_PX).toFixed(0)}px`);
    p.el.dataset.tone = tone;
    p.el.textContent = text;
    p.el.classList.remove(ANIM_CLASS);
    p.el.hidden = false;
    void p.el.offsetWidth; // restart the CSS animation
    p.el.classList.add(ANIM_CLASS);
  }

  update(nowMs: number): void {
    for (const p of this.pool) if (!p.el.hidden && nowMs > p.until) p.el.hidden = true;
  }
}
