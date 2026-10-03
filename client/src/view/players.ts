import * as THREE from "three";
import { THEME } from "../env/theme";
import { Character, CharacterPose, DEFAULT_LOOK, PART_TIER, StrikeTier, createCharacter } from "../character/character";
import { IMPACT_T } from "../character/strikes";
import { HitFlash } from "./hit-flash";
import {
  EventKind, Extra, MAX_PLAYERS, PART_NAMES, PLAYER_EXTRA, PLAYER_POS, Quality, SimEvent, Snap, teamOf,
} from "../snapshot";

const TEAM_SHIRTS = THEME.team;
const PARTNER_SHORTS = [0xf2f2f2, 0x2b2f36] as const; // slot 0 / slot 1 of a team
const TICK_RATE = 120; // mirrors sim.TickRate
const SPEED_REF = 7.5; // m/s mapped to pose.speed = 1 (top run speed)
const FACING_MIN_SPEED = 0.5; // below this, players face the net
const FACE_POS_X = Math.PI / 2; // facing +X (team 0 looks at the net)
const STRIKE_DURATION_S = 0.45;
const STRIKE_START_T = IMPACT_T * 0.5; // events arrive at impact: show the tail of the wind-up

const lerp = (a: number, b: number, t: number): number => a + (b - a) * t;

interface Slot {
  readonly character: Character;
  readonly pose: CharacterPose;
  readonly contact: THREE.Vector3;
  readonly flash: HitFlash;
  strikeT: number;
  strikeTier: StrikeTier;
  strikePerfect: boolean;
  hasContact: boolean;
}

function makeSlot(scene: THREE.Scene): Slot {
  const character = createCharacter(DEFAULT_LOOK);
  character.root.traverse((o) => { o.castShadow = true; });
  scene.add(character.root);
  const strike = { tier: "low" as StrikeTier, perfect: false, t: 0, contact: null };
  const pose: CharacterPose = {
    speed: 0, airborne: false, verticalVel: 0, charge: 0, strike, facing: FACE_POS_X,
  };
  return { character, pose, contact: new THREE.Vector3(), flash: new HitFlash(character.root), strikeT: 0, strikeTier: "low", strikePerfect: false, hasContact: false };
}

/** Up to 4 procedural characters posed from the interpolated snapshot. Allocation-free per frame. */
export class PlayersView {
  private readonly slots: Slot[];
  private lastNumPlayers = -1;

  constructor(scene: THREE.Scene) {
    this.slots = [];
    for (let i = 0; i < MAX_PLAYERS; i++) this.slots.push(makeSlot(scene));
  }

  /** Starts strike clips from EvKick/EvBodyHit/EvWhiff. */
  onEvent(e: SimEvent): void {
    if (e.kind !== EventKind.Kick && e.kind !== EventKind.BodyHit && e.kind !== EventKind.Whiff) return;
    if (e.player < 0 || e.player >= MAX_PLAYERS) return;
    const s = this.slots[e.player];
    const name = PART_NAMES[e.part] ?? "none";
    s.strikeTier = name === "none" ? "low" : PART_TIER[name];
    s.strikePerfect = e.kind === EventKind.Kick && e.quality === Quality.Perfect;
    s.strikeT = STRIKE_START_T;
    s.hasContact = e.kind !== EventKind.Whiff;
    s.contact.set(e.x, e.y, e.z);
  }

  /** Gold flash on player i (perfect hit). */
  flash(i: number): void {
    if (i >= 0 && i < MAX_PLAYERS) this.slots[i].flash.trigger();
  }

  update(prev: Float64Array, curr: Float64Array, alpha: number, dt: number): void {
    const n = curr[Snap.NumPlayers] | 0;
    if (n !== this.lastNumPlayers) this.recolor(n);
    for (let i = 0; i < MAX_PLAYERS; i++) {
      const s = this.slots[i];
      s.character.root.visible = i < n;
      if (i < n) this.place(i, s, prev, curr, alpha, dt, n);
      s.flash.update(dt);
    }
  }

  /** Interpolated world position of player i's feet into out. */
  feet(i: number, prev: Float64Array, curr: Float64Array, alpha: number, out: THREE.Vector3): THREE.Vector3 {
    const b = PLAYER_POS[i];
    return out.set(lerp(prev[b], curr[b], alpha), lerp(prev[b + 1], curr[b + 1], alpha), lerp(prev[b + 2], curr[b + 2], alpha));
  }

  private place(i: number, s: Slot, prev: Float64Array, curr: Float64Array, alpha: number, dt: number, n: number): void {
    this.feet(i, prev, curr, alpha, s.character.root.position);
    const e = PLAYER_EXTRA[i];
    const vx = curr[e + Extra.VelX];
    const vz = curr[e + Extra.VelZ];
    const speed = Math.hypot(vx, vz);
    const p = s.pose;
    p.speed = Math.min(1, speed / SPEED_REF);
    p.airborne = curr[e + Extra.OnGround] === 0;
    p.verticalVel = (curr[PLAYER_POS[i] + 1] - prev[PLAYER_POS[i] + 1]) * TICK_RATE;
    p.charge = curr[e + Extra.Charge];
    p.facing = speed > FACING_MIN_SPEED ? Math.atan2(vx, vz) : teamOf(i, n) === 0 ? FACE_POS_X : -FACE_POS_X;
    this.advanceStrike(s, dt);
    s.character.update(p, dt);
  }

  private advanceStrike(s: Slot, dt: number): void {
    const st = s.pose.strike;
    if (!st) return;
    if (s.strikeT > 0) s.strikeT += dt / STRIKE_DURATION_S;
    if (s.strikeT >= 1) s.strikeT = 0;
    st.t = s.strikeT;
    st.tier = s.strikeTier;
    st.perfect = s.strikePerfect;
    st.contact = s.strikeT > 0 && s.hasContact ? s.contact : null;
  }

  /** Team colors depend on the mode's index->team mapping; only runs when NumPlayers changes. */
  private recolor(n: number): void {
    this.lastNumPlayers = n;
    for (let i = 0; i < MAX_PLAYERS; i++) {
      const team = teamOf(i, n);
      const slotInTeam = n >= MAX_PLAYERS ? i % 2 : 0;
      this.slots[i].character.setLook({ ...DEFAULT_LOOK, shirt: TEAM_SHIRTS[team & 1], shorts: PARTNER_SHORTS[slotInTeam] });
    }
  }
}
