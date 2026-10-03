import { DEMO_STATES, DemoState, SideMode } from "./preview-driver";

// DOM controls for the character preview page.

export interface PreviewControls {
  state: DemoState;
  speed: number;
  showHitbox: boolean;
  closeupAngle: number;
  /** Frozen strike time 0..1, or null to play the strike loop. */
  scrub: number | null;
  paused: boolean;
  side: SideMode;
}

const CLOSEUP_ANGLES = 4;

function button(label: string, onClick: () => void): HTMLButtonElement {
  const b = document.createElement("button");
  b.textContent = label;
  b.addEventListener("click", onClick);
  return b;
}

function stateButtons(ctl: PreviewControls, host: HTMLElement): void {
  const row = document.createElement("div");
  row.className = "row";
  const buttons = DEMO_STATES.map((s) => {
    const b = button(s, () => {
      ctl.state = s;
      for (const other of buttons) other.classList.toggle("on", other === b);
    });
    b.classList.toggle("on", s === ctl.state);
    return b;
  });
  for (const b of buttons) row.appendChild(b);
  host.appendChild(row);
}

function slider(host: HTMLElement, name: string, value: number, onInput: (v: number) => void): HTMLInputElement {
  const label = document.createElement("label");
  const input = document.createElement("input");
  input.type = "range";
  input.min = "0";
  input.max = "1";
  input.step = "0.01";
  input.value = String(value);
  const out = document.createElement("span");
  out.textContent = value.toFixed(2);
  input.addEventListener("input", () => {
    const v = Number(input.value);
    out.textContent = v.toFixed(2);
    onInput(v);
  });
  label.append(`${name} `, input, out);
  host.appendChild(label);
  return input;
}

function scrubControls(ctl: PreviewControls, row: HTMLElement): void {
  const auto = button("t: auto", () => {
    ctl.scrub = null;
    auto.classList.add("on");
  });
  auto.classList.add("on");
  slider(row, "t", 0, (v) => {
    ctl.scrub = v;
    auto.classList.remove("on");
  });
  row.appendChild(auto);
  const pause = button("pause", () => {
    ctl.paused = !ctl.paused;
    pause.classList.toggle("on", ctl.paused);
  });
  row.appendChild(pause);
  const modes: readonly SideMode[] = ["auto", "R", "L"];
  const side = button("side: auto", () => {
    ctl.side = modes[(modes.indexOf(ctl.side) + 1) % modes.length];
    side.textContent = `side: ${ctl.side}`;
  });
  row.appendChild(side);
}

export function buildControls(host: HTMLElement, info: string): PreviewControls {
  const ctl: PreviewControls = {
    state: "idle", speed: 0.6, showHitbox: true, closeupAngle: 0, scrub: null, paused: false, side: "auto",
  };
  stateButtons(ctl, host);
  const row = document.createElement("div");
  row.className = "row";
  slider(row, "speed", ctl.speed, (v) => { ctl.speed = v; });
  scrubControls(ctl, row);
  const hit = button("hitbox + zones", () => {
    ctl.showHitbox = !ctl.showHitbox;
    hit.classList.toggle("on", ctl.showHitbox);
  });
  hit.classList.add("on");
  row.appendChild(hit);
  row.appendChild(button("close-up angle", () => {
    ctl.closeupAngle = (ctl.closeupAngle + 1) % CLOSEUP_ANGLES;
  }));
  const stats = document.createElement("span");
  stats.className = "info";
  stats.textContent = info;
  row.appendChild(stats);
  host.appendChild(row);
  return ctl;
}
