import { t } from "./i18n/t";
import { keyHint } from "./ui/key-hints";
import type { KeyAction } from "./ui/keybinds";
import { Extra, FLAG_LOB, PLAYER_DASH, PLAYER_EXTRA } from "./snapshot";

// Bottom-center action bar (Dash · Golpe · Globo; jump stays on its key but is not
// shown, like most games): round ability buttons with SVG icons, a progress
// ring (cooldown for Dash, charge for Golpe) and the key label underneath.
// Reads the human's extras from the snapshot; touches the DOM only on change
// (rings are quantized so idle frames write nothing). Dash's ring is its
// cooldown radial (fill = ready fraction, supplied by the HUD: the snapshot
// only carries DashActive, not the cooldown counter).
const RING_R = 25;
const RING_C = 2 * Math.PI * RING_R;
const RING_STEPS = 60;
const SVG_NS = "http://www.w3.org/2000/svg";

type ActionId = "dash" | "strike" | "lob";

/** `bind` = remappable keyboard action (label follows Ajustes), else the fixed `key` text. */
interface ActionDef {
  readonly id: ActionId; readonly label: string; readonly key: string; readonly bind?: KeyAction; readonly icon: string; readonly ring: boolean;
}

const ICONS: Readonly<Record<ActionId, string>> = {
  strike: "M32 18 a14 14 0 1 0 0.01 0 Z M14 32 H20 M44 32 H50 M32 14 V8",
  lob: "M16 44 Q32 6 48 44 M42 40 L48 44 L50 37",
  dash: "M16 24 H30 M12 32 H28 M16 40 H30 M34 20 L48 32 L34 44",
};

const ACTIONS: readonly ActionDef[] = [
  { id: "dash", label: "Dash", key: "Q", bind: "dash", icon: ICONS.dash, ring: true },
  { id: "strike", label: t("Golpe", "Strike"), key: "Click", icon: ICONS.strike, ring: true },
  { id: "lob", label: t("Globo", "Lob"), key: t("Click der", "Right click"), icon: ICONS.lob, ring: false },
];

interface Slot {
  readonly root: HTMLDivElement;
  readonly key: HTMLElement;
  readonly bind: KeyAction | undefined;
  readonly ring: SVGCircleElement | null;
  lastRing: number;
  lastActive: boolean;
  lastWarn: boolean;
}

function svg<K extends keyof SVGElementTagNameMap>(tag: K, attrs: Readonly<Record<string, string>>): SVGElementTagNameMap[K] {
  const e = document.createElementNS(SVG_NS, tag);
  for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, v);
  return e;
}

function button(def: ActionDef): Slot {
  const root = document.createElement("div");
  root.className = `act act--${def.id}`;
  const icon = svg("svg", { viewBox: "0 0 64 64", class: "act__svg", "aria-hidden": "true" });
  icon.append(svg("circle", { cx: "32", cy: "32", r: String(RING_R), class: "act__track" }));
  let ring: SVGCircleElement | null = null;
  if (def.ring) {
    ring = svg("circle", {
      cx: "32", cy: "32", r: String(RING_R), class: "act__ring",
      "stroke-dasharray": RING_C.toFixed(2), "stroke-dashoffset": RING_C.toFixed(2), transform: "rotate(-90 32 32)",
    });
    icon.append(ring);
  }
  icon.append(svg("path", { d: def.icon, class: "act__icon" }));
  const label = document.createElement("span");
  label.className = "act__label";
  label.textContent = def.label;
  const key = document.createElement("kbd");
  key.className = "act__key";
  key.textContent = def.bind ? keyHint(def.bind) : def.key;
  root.append(icon, label, key);
  return { root, key, bind: def.bind, ring, lastRing: -1, lastActive: false, lastWarn: false };
}

export class ActionBar {
  readonly root = document.createElement("div");
  private readonly slots: Readonly<Record<ActionId, Slot>>;

  constructor() {
    this.root.id = "actions";
    const made = ACTIONS.map(button);
    this.slots = { dash: made[0], strike: made[1], lob: made[2] };
    this.root.append(...made.map((s) => s.root));
  }

  refreshKeys(): void {
    for (const s of Object.values(this.slots)) if (s.bind) s.key.textContent = keyHint(s.bind);
  }

  /** humanSlot's extras block; called every frame, writes only on change. dashReady: 0..1 cooldown progress. */
  update(snap: Float64Array, humanSlot: number, dashReady: number): void {
    const e = PLAYER_EXTRA[humanSlot];
    const flags = snap[e + Extra.Flags] | 0;
    const charge = snap[e + Extra.Charge];
    this.set(this.slots.dash, dashReady, snap[PLAYER_DASH[humanSlot]] === 1, false);
    this.set(this.slots.strike, charge, charge > 0, false);
    this.set(this.slots.lob, 0, (flags & FLAG_LOB) !== 0, false);
  }

  private set(s: Slot, fill: number, active: boolean, warn: boolean): void {
    if (s.ring) {
      const q = Math.round(Math.max(0, Math.min(1, fill)) * RING_STEPS);
      if (q !== s.lastRing) {
        s.lastRing = q;
        s.ring.setAttribute("stroke-dashoffset", (RING_C * (1 - q / RING_STEPS)).toFixed(2));
      }
    }
    if (active !== s.lastActive) {
      s.lastActive = active;
      s.root.classList.toggle("is-active", active);
    }
    if (warn !== s.lastWarn) {
      s.lastWarn = warn;
      s.root.classList.toggle("is-warn", warn);
    }
  }
}
