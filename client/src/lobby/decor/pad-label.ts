import * as THREE from "three";
import { THEME } from "../../env/theme";
import { FONT_STACK, canvasTexture } from "./util";

// Big floating mode label above each pad (destination marker, drawn once):
// huge glowing title + small subtitle. Animated by the caller (bob/scale).
const W = 512;
const H = 320;
const WORLD_W = 3.6;
const TITLE_PX = 170;
const SUB_PX = 62;
const GLOW = 12;
const DIM = 0.72; // keeps the text under the bloom threshold: crisp, not washed

const css = (hex: number): string => `#${hex.toString(16).padStart(6, "0")}`;

export function padLabel(title: string, sub: string, color: number): THREE.Sprite {
  const accent = css(color);
  const tex = canvasTexture(W, H, (ctx) => {
    ctx.clearRect(0, 0, W, H);
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
    ctx.font = `${TITLE_PX}px ${FONT_STACK}`;
    ctx.shadowColor = accent;
    ctx.shadowBlur = GLOW;
    ctx.lineWidth = 10;
    ctx.strokeStyle = THEME.lobbyDecor.screenBg;
    ctx.strokeText(title, W / 2, H * 0.42, W - 20);
    ctx.fillStyle = THEME.lobbyDecor.padLabelText;
    ctx.fillText(title, W / 2, H * 0.42, W - 20);
    ctx.font = `${SUB_PX}px ${FONT_STACK}`;
    ctx.shadowBlur = 0;
    ctx.lineWidth = 8;
    ctx.strokeText(sub, W / 2, H * 0.86, W - 20);
    ctx.fillStyle = accent;
    ctx.fillText(sub, W / 2, H * 0.86, W - 20);
  });
  const sprite = new THREE.Sprite(new THREE.SpriteMaterial({ map: tex, transparent: true, depthWrite: false, color: new THREE.Color(DIM, DIM, DIM) }));
  sprite.scale.set(WORLD_W, (WORLD_W * H) / W, 1);
  return sprite;
}
