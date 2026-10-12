import * as THREE from "three";
import { Character, CharacterPose, CharacterStyle, createCharacter, styleFromSeed } from "../character/character";
import { BodyReaction, EMOTES, EMOTE_S } from "../view/body-reaction";
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
  readonly body: BodyReaction;
  facing: number;
  id: number;
  name: string;
  lastY: number;
  seenAt: number;
}

export class RemotePlayers {
  readonly group = new THREE.Group();
  private readonly byId = new Map<number, Remote>();
  /** Same remotes as byId, iterated by index every frame (no iterator allocations). */
  private readonly active: Remote[] = [];
  private readonly free: Remote[] = [];
  /** Names and looks that arrived before the first position (join order on the server). */
  private readonly pendingNames = new Map<number, string>();
  private readonly pendingLooks = new Map<number, CharacterStyle>();
  /** Players this viewer hid ("Jugadores en el lobby"); lobby ids live as long as their connection. */
  private readonly hidden = new Set<number>();

  /** Everyone in view, for the players panel (not per frame). */
  list(): Array<{ id: number; name: string; hidden: boolean }> {
    return this.active.map((r) => ({ id: r.id, name: r.name, hidden: this.hidden.has(r.id) }));
  }

  /** Hides (or shows again) a player's puppet and name for this viewer only. */
  setHidden(id: number, on: boolean): void {
    if (on) this.hidden.add(id);
    else this.hidden.delete(id);
    const r = this.byId.get(id);
    if (r) r.ch.root.visible = r.tag.visible = !on;
  }

  /** Roster entry: create on first sight, then only retarget. */
  place(id: number, x: number, y: number, z: number, facing: number): void {
    let r = this.byId.get(id);
    if (!r) {
      r = this.free.pop() ?? this.build();
      r.id = id;
      r.ch.root.position.set(x, y, z);
      r.ch.root.visible = r.tag.visible = !this.hidden.has(id);
      this.byId.set(id, r);
      this.active.push(r);
      this.setTag(r, this.pendingNames.get(id) ?? "");
      this.pendingNames.delete(id);
      const look = this.pendingLooks.get(id);
      if (look) this.wear(r, look);
      this.pendingLooks.delete(id);
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

  /** Their own look (Estilos), with their shirt number. */
  look(id: number, st: CharacterStyle): void {
    const r = this.byId.get(id);
    if (r) this.wear(r, st);
    else this.pendingLooks.set(id, st);
  }

  /** A lobby emote (wire kind 0..3, view/body-reaction.ts EMOTES). */
  emote(id: number, kind: number): void {
    const r = this.byId.get(id);
    const k = EMOTES[kind];
    if (r && k) r.body.trigger(k, EMOTE_S);
  }

  private wear(r: Remote, st: CharacterStyle): void {
    r.ch.setStyle(st);
    r.ch.setJersey(st.number, "");
  }

  remove(id: number): void {
    const r = this.byId.get(id);
    this.pendingNames.delete(id);
    this.pendingLooks.delete(id);
    if (!r) return;
    this.byId.delete(id);
    const i = this.active.indexOf(r);
    this.active[i] = this.active[this.active.length - 1];
    this.active.pop();
    r.ch.root.visible = r.tag.visible = false;
    r.body.reset();
    r.ch.setStyle(styleFromSeed(301 + this.free.length * 7)); // the next one starts anonymous
    this.free.push(r);
  }

  clear(): void {
    while (this.active.length > 0) this.remove(this.active[this.active.length - 1].id);
    this.pendingNames.clear();
    this.pendingLooks.clear();
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
      r.body.update(r.pose, dt);
      r.tag.position.set(p.x, ny + TAG_Y, p.z);
      r.ch.update(r.pose, dt);
    }
  }

  private build(): Remote {
    const i = this.byId.size + this.free.length;
    const D = THEME.lobbyDecor;
    const ch = createCharacter({
      shirt: D.npcShirts[(i + 2) % D.npcShirts.length], shorts: THEME.crowdBody[i % THEME.crowdBody.length],
      trim: THEME.crowdBody[i % THEME.crowdBody.length], ink: THEME.crowdBody[i % THEME.crowdBody.length],
      skin: THEME.crowdSkin[i % THEME.crowdSkin.length], hair: THEME.crowdBody[(i + 3) % THEME.crowdBody.length], boots: D.neon[i % D.neon.length],
    });
    ch.setStyle(styleFromSeed(301 + i * 7)); // until their own look arrives (LOOK)
    const tag = new THREE.Sprite(new THREE.SpriteMaterial({ transparent: true, depthWrite: false }));
    tag.scale.set(TAG_W, (TAG_W * TAG_PX_H) / TAG_PX_W, 1);
    this.group.add(ch.root, tag);
    return { ch, tag, target: new THREE.Vector3(), body: new BodyReaction(), facing: 0, id: -1, name: "", lastY: 0, seenAt: 0,
      pose: { speed: 0, airborne: false, verticalVel: 0, charge: 0, strike: null, facing: 0 } };
  }

  private setTag(r: Remote, text: string): void {
    r.name = text;
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
