import * as THREE from "three";
import { Character, CharacterPose, createCharacter } from "../character/character";
import { THEME } from "../env/theme";
import { canvasTexture, FONT_STACK } from "./decor/util";

// Other players in the shared online lobby: same puppet as everyone, smoothed
// toward the last reported position (10 Hz) with a name tag. A character is
// built the first time an id appears (not per frame) and reused after it leaves.
const FOLLOW_RATE = 12; // 1/s: exponential approach to the reported position
const WALK_SPEED_NORM = 6; // m/s mapped to pose.speed 1 (lobby/decor/npcs.ts)
const AIR_Y = 0.05;
const TAG_Y = 2.45;
const TAG_W = 1.6;
const TAG_PX_W = 256;
const TAG_PX_H = 64;
const STALE_MS = 2000; // not in a roster for this long (missed GONE): drop it

interface Remote {
  readonly ch: Character;
  readonly pose: CharacterPose;
  readonly tag: THREE.Sprite;
  readonly target: THREE.Vector3;
  facing: number;
  id: number;
  lastY: number;
  seenAt: number;
}

export class RemotePlayers {
  readonly group = new THREE.Group();
  private readonly byId = new Map<number, Remote>();
  /** Same remotes as byId, iterated by index every frame (no iterator allocations). */
  private readonly active: Remote[] = [];
  private readonly free: Remote[] = [];
  /** Names that arrived before the first position (join order on the server). */
  private readonly pendingNames = new Map<number, string>();

  /** Roster entry: create on first sight, then only retarget. */
  place(id: number, x: number, y: number, z: number, facing: number): void {
    let r = this.byId.get(id);
    if (!r) {
      r = this.free.pop() ?? this.build();
      r.id = id;
      r.ch.root.position.set(x, y, z);
      r.ch.root.visible = r.tag.visible = true;
      this.byId.set(id, r);
      this.active.push(r);
      this.setTag(r, this.pendingNames.get(id) ?? "");
      this.pendingNames.delete(id);
    }
    r.target.set(x, y, z);
    r.facing = facing;
    r.seenAt = performance.now();
  }

  name(id: number, text: string): void {
    const r = this.byId.get(id);
    if (r) this.setTag(r, text);
    else this.pendingNames.set(id, text);
  }

  remove(id: number): void {
    const r = this.byId.get(id);
    this.pendingNames.delete(id);
    if (!r) return;
    this.byId.delete(id);
    const i = this.active.indexOf(r);
    this.active[i] = this.active[this.active.length - 1];
    this.active.pop();
    r.ch.root.visible = r.tag.visible = false;
    this.free.push(r);
  }

  clear(): void {
    while (this.active.length > 0) this.remove(this.active[this.active.length - 1].id);
    this.pendingNames.clear();
  }

  update(dt: number): void {
    if (!(dt > 0)) return;
    const k = 1 - Math.exp(-FOLLOW_RATE * dt);
    const now = performance.now();
    for (let i = this.active.length - 1; i >= 0; i--) {
      const r = this.active[i];
      if (now - r.seenAt > STALE_MS) { this.remove(r.id); continue; }
      const p = r.ch.root.position;
      const dx = (r.target.x - p.x) * k;
      const dz = (r.target.z - p.z) * k;
      const ny = p.y + (r.target.y - p.y) * k;
      r.pose.verticalVel = (ny - r.lastY) / dt;
      r.lastY = ny;
      p.set(p.x + dx, ny, p.z + dz);
      r.pose.speed = Math.min(1, Math.hypot(dx, dz) / dt / WALK_SPEED_NORM);
      r.pose.airborne = ny > AIR_Y;
      r.pose.facing = r.facing;
      r.tag.position.set(p.x, ny + TAG_Y, p.z);
      r.ch.update(r.pose, dt);
    }
  }

  private build(): Remote {
    const i = this.byId.size + this.free.length;
    const D = THEME.lobbyDecor;
    const ch = createCharacter({
      shirt: D.npcShirts[(i + 2) % D.npcShirts.length], shorts: THEME.crowdBody[i % THEME.crowdBody.length],
      skin: THEME.crowdSkin[i % THEME.crowdSkin.length], hair: THEME.crowdBody[(i + 3) % THEME.crowdBody.length], boots: D.neon[i % D.neon.length],
    });
    const tag = new THREE.Sprite(new THREE.SpriteMaterial({ transparent: true, depthWrite: false }));
    tag.scale.set(TAG_W, (TAG_W * TAG_PX_H) / TAG_PX_W, 1);
    this.group.add(ch.root, tag);
    return { ch, tag, target: new THREE.Vector3(), facing: 0, id: -1, lastY: 0, seenAt: 0,
      pose: { speed: 0, airborne: false, verticalVel: 0, charge: 0, strike: null, facing: 0 } };
  }

  private setTag(r: Remote, text: string): void {
    const D = THEME.lobbyDecor;
    r.tag.material.map?.dispose();
    r.tag.material.map = canvasTexture(TAG_PX_W, TAG_PX_H, (ctx) => {
      ctx.clearRect(0, 0, TAG_PX_W, TAG_PX_H);
      ctx.font = `28px ${FONT_STACK}`;
      ctx.textAlign = "center";
      ctx.textBaseline = "middle";
      ctx.lineWidth = 6;
      ctx.strokeStyle = D.nameTagStroke;
      ctx.strokeText(text, TAG_PX_W / 2, TAG_PX_H / 2, TAG_PX_W - 8);
      ctx.fillStyle = D.nameTagText;
      ctx.fillText(text, TAG_PX_W / 2, TAG_PX_H / 2, TAG_PX_W - 8);
    });
    r.tag.material.needsUpdate = true;
  }
}
