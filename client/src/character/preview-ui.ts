import { DEMO_STATES, DemoState } from "./preview-driver";

// DOM controls for the character preview page.

export interface PreviewControls {
  state: DemoState;
  speed: number;
  showHitbox: boolean;
  closeupAngle: number;
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

function speedSlider(ctl: PreviewControls, host: HTMLElement): void {
  const label = document.createElement("label");
  const input = document.createElement("input");
  input.type = "range";
  input.min = "0";
  input.max = "1";
  input.step = "0.01";
  input.value = String(ctl.speed);
  const out = document.createElement("span");
  out.textContent = ctl.speed.toFixed(2);
  input.addEventListener("input", () => {
    ctl.speed = Number(input.value);
    out.textContent = ctl.speed.toFixed(2);
  });
  label.append("speed ", input, out);
  host.appendChild(label);
}

export function buildControls(host: HTMLElement, info: string): PreviewControls {
  const ctl: PreviewControls = { state: "idle", speed: 0.6, showHitbox: true, closeupAngle: 0 };
  stateButtons(ctl, host);
  const row = document.createElement("div");
  row.className = "row";
  speedSlider(ctl, row);
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
