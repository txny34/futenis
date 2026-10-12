import * as THREE from "three";
import { Character, CharacterPose, createCharacter, styleFromSeed } from "../../character/character";
import { THEME } from "../../env/theme";

// Decorative lobby NPCs (no collision, no sim): some stroll on ellipses, some
// idle near the pads and hop now and then. Same rig as the players; only the
// look changes. Allocation-free update.
const TAU = Math.PI * 2;
const HOP_H = 0.35;
const HOP_S = 0.5;
const HOP_EVERY_S = 9;
const WALK_SPEED_NORM = 6; // m/s mapped to pose.speed 1

interface Walker { readonly cx: number; readonly cz: number; readonly rx: number; readonly rz: number; readonly w: number; readonly phase: number }
interface Idler { readonly x: number; readonly z: number; readonly facing: number; readonly phase: number }
type Spec = { readonly walk: Walker } | { readonly idle: Idler };

const SPECS: readonly Spec[] = [
  { walk: { cx: -11, cz: 4, rx: 3.2, rz: 2.4, w: 0.07, phase: 0 } },
  { walk: { cx: 11, cz: 3, rx: 3.6, rz: 2.2, w: -0.06, phase: 2 } },
  { walk: { cx: 0, cz: -10.8, rx: 7, rz: 0.6, w: 0.035, phase: 1 } },
  { idle: { x: -9.2, z: -6.6, facing: 0.89, phase: 0.4 } },
  { idle: { x: 9.4, z: -6.8, facing: -0.88, phase: 1.9 } },
  { idle: { x: 3.2, z: -8.6, facing: -0.89, phase: 1.1 } },
];

interface Npc { readonly ch: Character; readonly pose: CharacterPose; readonly spec: Spec; lastY: number }

export class Npcs {
  readonly group = new THREE.Group();
  private readonly npcs: Npc[] = [];
  private time = 0;

  constructor() {
    const D = THEME.lobbyDecor;
    SPECS.forEach((spec, i) => {
      const ch = createCharacter({
        shirt: D.npcShirts[i % D.npcShirts.length], shorts: THEME.crowdBody[i % THEME.crowdBody.length],
        skin: THEME.crowdSkin[i % THEME.crowdSkin.length], hair: THEME.crowdBody[(i + 3) % THEME.crowdBody.length], boots: D.neon[i % D.neon.length],
      });
      ch.setStyle(styleFromSeed(101 + i * 13)); // a crowd of different faces and hairdos
      this.group.add(ch.root);
      this.npcs.push({ ch, spec, lastY: 0, pose: { speed: 0, airborne: false, verticalVel: 0, charge: 0, strike: null, facing: 0 } });
    });
  }

  update(dt: number): void {
    if (!(dt > 0)) return;
    this.time += dt;
    for (const n of this.npcs) {
      if ("walk" in n.spec) this.walk(n, n.spec.walk);
      else this.idle(n, n.spec.idle, dt);
      n.ch.update(n.pose, dt);
    }
  }

  private walk(n: Npc, w: Walker): void {
    const a = this.time * w.w * TAU + w.phase;
    const vx = -Math.sin(a) * w.rx * w.w * TAU;
    const vz = Math.cos(a) * w.rz * w.w * TAU;
    n.ch.root.position.set(w.cx + Math.cos(a) * w.rx, 0, w.cz + Math.sin(a) * w.rz);
    n.pose.facing = Math.atan2(vx, vz);
    n.pose.speed = Math.min(1, Math.hypot(vx, vz) / WALK_SPEED_NORM);
  }

  private idle(n: Npc, s: Idler, dt: number): void {
    const t = (this.time + s.phase * HOP_EVERY_S) % HOP_EVERY_S;
    const y = t < HOP_S ? Math.sin((t / HOP_S) * Math.PI) * HOP_H : 0;
    n.ch.root.position.set(s.x, y, s.z);
    n.pose.airborne = y > 0;
    n.pose.verticalVel = (y - n.lastY) / dt;
    n.lastY = y;
    n.pose.facing = s.facing;
    n.pose.speed = 0;
  }
}
