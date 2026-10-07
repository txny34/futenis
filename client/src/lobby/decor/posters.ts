import * as THREE from "three";
import { THEME } from "../../env/theme";
import { FONT_STACK, canvasTexture } from "./util";

// Posters/banners on the side walls: canvas textures drawn once at load.
const W = 384;
const H = 576;
const WORLD_W = 2.6;
const Y = 4.2;
const WALL_X = 19.3;
const ZS: readonly number[] = [-8, -1, 6];
const D = THEME.lobbyDecor;
const POSTER_DIM = 0.4;

interface PosterSpec { readonly title: string; readonly sub: string }
const SPECS: readonly PosterSpec[] = [
  { title: "1v1", sub: "CONTRA EL BOT" }, { title: "2v2", sub: "EN EQUIPO" }, { title: "ENTRENÁ", sub: "LANZAPELOTAS" },
  { title: "UN TOQUE", sub: "POR EQUIPO" }, { title: "PAREDES", sub: "SON TUYAS" }, { title: "ARENA", sub: "NOCTURNA NEÓN" },
];

function draw(ctx: CanvasRenderingContext2D, spec: PosterSpec, i: number): void {
  const accent = i % 2 === 0 ? D.screenAccentA : D.screenAccentB;
  const g = ctx.createLinearGradient(0, 0, 0, H);
  g.addColorStop(0, D.posterBg[i % D.posterBg.length]);
  g.addColorStop(1, D.screenBg);
  ctx.fillStyle = g;
  ctx.fillRect(0, 0, W, H);
  ctx.strokeStyle = accent;
  ctx.lineWidth = 10;
  ctx.shadowColor = accent;
  ctx.shadowBlur = 20;
  ctx.strokeRect(14, 14, W - 28, H - 28);
  ctx.beginPath(); // stylized ball arc
  ctx.arc(W / 2, H * 0.36, 90, 0, Math.PI * 2);
  ctx.stroke();
  ctx.beginPath();
  ctx.arc(W / 2 - 60, H * 0.36, 90, -0.9, 0.9);
  ctx.stroke();
  ctx.shadowBlur = 0;
  ctx.fillStyle = D.screenText;
  ctx.textAlign = "center";
  ctx.font = `72px ${FONT_STACK}`;
  ctx.fillText(spec.title, W / 2, H * 0.72, W - 40);
  ctx.fillStyle = accent;
  ctx.font = `30px ${FONT_STACK}`;
  ctx.fillText(spec.sub, W / 2, H * 0.82, W - 40);
}

export function buildPosters(): THREE.Group {
  const g = new THREE.Group();
  const geo = new THREE.PlaneGeometry(WORLD_W, (WORLD_W * H) / W);
  SPECS.forEach((spec, i) => {
    const side = i < ZS.length ? -1 : 1;
    const tex = canvasTexture(W, H, (ctx) => draw(ctx, spec, i));
    const m = new THREE.Mesh(geo, new THREE.MeshBasicMaterial({ map: tex, color: new THREE.Color(POSTER_DIM, POSTER_DIM, POSTER_DIM) }));
    m.position.set(side * WALL_X, Y, ZS[i % ZS.length]);
    m.rotation.y = (-side * Math.PI) / 2;
    g.add(m);
  });
  return g;
}
