import { Extra, FLAG_LOB, FLAG_STAMINA_LOCK, PLAYER_DASH, PLAYER_EXTRA } from "./snapshot";

// Bottom-center action bar: round ability buttons with SVG icons, a progress
// ring (stamina for Sprint, charge for Golpe) and the key label underneath.
// Reads the human's extras from the snapshot; touches the DOM only on change
// (rings are quantized so idle frames write nothing). Dash's ring is its
// cooldown radial (fill = ready fraction, supplied by the HUD: the snapshot
// only carries DashActive, not the cooldown counter).
const RING_R = 25;
const RING_C = 2 * Math.PI * RING_R;
const RING_STEPS = 60;
const SPRINT_SPEED = 6; // m/s: above this we light the Sprint button
const SVG_NS = "http://www.w3.org/2000/svg";

type ActionId = "sprint" | "dash" | "strike" | "lob" | "jump";

interface ActionDef { readonly id: ActionId; readonly label: string; readonly key: string; readonly icon: string; readonly ring: boolean }

const ICONS: Readonly<Record<ActionId, string>> = {
  sprint: "M34 14 L22 33 H30 L26 50 L42 28 H33 Z",
  strike: "M32 18 a14 14 0 1 0 0.01 0 Z M14 32 H20 M44 32 H50 M32 14 V8",
  lob: "M16 44 Q32 6 48 44 M42 40 L48 44 L50 37",
  jump: "M32 46 V20 M22 30 L32 18 L42 30 M22 50 H42",
  dash: "M16 24 H30 M12 32 H28 M16 40 H30 M34 20 L48 32 L34 44",
};

const ACTIONS: readonly ActionDef[] = [
  { id: "sprint", label: "Sprint", key: "Shift", icon: ICONS.sprint, ring: true },
  { id: "dash", label: "Dash", key: "Q", icon: ICONS.dash, ring: true },
  { id: "strike", label: "Golpe", key: "Click", icon: ICONS.strike, ring: true },
  { id: "lob", label: "Globo", key: "Click der", icon: ICONS.lob, ring: false },
  { id: "jump", label: "Salto", key: "Espacio", icon: ICONS.jump, ring: false },
];

interface Slot {
  readonly root: HTMLDivElement;
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
  key.textContent = def.key;
  root.append(icon, label, key);
  return { root, ring, lastRing: -1, lastActive: false, lastWarn: false };
}

export class ActionBar {
  readonly root = document.createElement("div");
  private readonly slots: Readonly<Record<ActionId, Slot>>;

  constructor(spinChip: HTMLElement) {
    this.root.id = "actions";
    const made = ACTIONS.map(button);
    this.slots = { sprint: made[0], dash: made[1], strike: made[2], lob: made[3], jump: made[4] };
    spinChip.classList.add("act", "act-spin");
    this.root.append(...made.map((s) => s.root), spinChip);
  }

  /** humanSlot's extras block; called every frame, writes only on change. dashReady: 0..1 cooldown progress. */
  update(snap: Float64Array, humanSlot: number, dashReady: number): void {
    const e = PLAYER_EXTRA[humanSlot];
    const flags = snap[e + Extra.Flags] | 0;
    const speed = Math.hypot(snap[e + Extra.VelX], snap[e + Extra.VelZ]);
    const charge = snap[e + Extra.Charge];
    this.set(this.slots.sprint, snap[e + Extra.Stamina], speed > SPRINT_SPEED, (flags & FLAG_STAMINA_LOCK) !== 0);
    this.set(this.slots.dash, dashReady, snap[PLAYER_DASH[humanSlot]] === 1, false);
    this.set(this.slots.strike, charge, charge > 0, false);
    this.set(this.slots.lob, 0, (flags & FLAG_LOB) !== 0, false);
    this.set(this.slots.jump, 0, snap[e + Extra.OnGround] === 0, false);
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
