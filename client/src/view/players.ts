import * as THREE from "three";
import { THEME } from "../env/theme";
import { Character, CharacterPose, DEFAULT_LOOK, PART_TIER, StrikeTier, createCharacter, styleFromSeed } from "../character/character";
import { loadStyle, onStyleChange } from "../profile/style-store";
import { IMPACT_T } from "../character/strikes";
import { HitFlash } from "./hit-flash";
import { moodFor } from "./player-moods";
import { BodyReaction, reactionFor } from "./body-reaction";
import {
  EventKind, Extra, MAX_PLAYERS, Mode, PART_NAMES, PLAYER_EXTRA, PLAYER_POS, Quality, SimEvent, Snap, teamOf,
} from "../snapshot";

const TEAM_SHIRTS = THEME.team;
const PARTNER_SHORTS = THEME.fx.partnerShorts; // slot 0 / slot 1 of a team
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
  readonly body: BodyReaction;
  strikeT: number;
  strikeTier: StrikeTier;
  strikePerfect: boolean;
  hasContact: boolean;
}

const HUMAN_SLOT = 0; // the local player is always slot 0 (online frames are rotated server-side)
const BOT_STYLE_SEED = 17; // varied, stable looks for the other slots

function makeSlot(scene: THREE.Scene, i: number): Slot {
  const character = createCharacter(DEFAULT_LOOK);
  character.setStyle(i === HUMAN_SLOT ? loadStyle() : styleFromSeed(BOT_STYLE_SEED + i * 31));
  character.root.traverse((o) => { o.castShadow = true; });
  scene.add(character.root);
  const strike = { tier: "low" as StrikeTier, perfect: false, t: 0, contact: null };
  const pose: CharacterPose = {
    speed: 0, airborne: false, verticalVel: 0, charge: 0, strike, facing: FACE_POS_X,
  };
  return { character, pose, contact: new THREE.Vector3(), flash: new HitFlash(character.root), body: new BodyReaction(), strikeT: 0, strikeTier: "low", strikePerfect: false, hasContact: false };
}

/** Up to 4 procedural characters posed from the interpolated snapshot. Allocation-free per frame. */
export class PlayersView {
  private readonly slots: Slot[];
  private lastNumPlayers = -1;
  private readonly ball = new THREE.Vector3(); // every player keeps an eye on it (character/look.ts)
  private ballDead = false;

  constructor(scene: THREE.Scene) {
    this.slots = [];
    for (let i = 0; i < MAX_PLAYERS; i++) this.slots.push(makeSlot(scene, i));
    onStyleChange((st) => this.slots[HUMAN_SLOT].character.setStyle(st)); // Estilos edits show at once
  }

  /** Starts strike clips from EvKick/EvBodyHit/EvWhiff (a body graze on a dead ball is ignored). */
  onEvent(e: SimEvent): void {
    this.trackBall(e.kind);
    this.react(e);
    if (e.kind !== EventKind.Kick && e.kind !== EventKind.BodyHit && e.kind !== EventKind.Whiff) return;
    if (e.kind === EventKind.BodyHit && this.ballDead) return;
    if (e.player < 0 || e.player >= MAX_PLAYERS) return;
    const s = this.slots[e.player];
    const name = PART_NAMES[e.part] ?? "none";
    s.strikeTier = name === "none" ? "low" : PART_TIER[name];
    s.strikePerfect = e.kind === EventKind.Kick && e.quality === Quality.Perfect;
    s.strikeT = STRIKE_START_T;
    s.hasContact = e.kind !== EventKind.Whiff;
    s.contact.set(e.x, e.y, e.z);
  }

  /** Faces for the event: every active player reacts by their team (player-moods.ts). */
  private react(e: SimEvent): void {
    const n = Math.max(this.lastNumPlayers, 0);
    for (let i = 0; i < n; i++) {
      const mood = moodFor(e, i, teamOf(i, n));
      if (mood !== null) this.slots[i].character.express(mood.emotion, mood.holdS);
      const body = reactionFor(e, teamOf(i, n));
      if (body !== null) this.slots[i].body.trigger(body.kind, body.seconds);
    }
  }

  /** Dead between a point/fault and the next toss or machine feed. No reset needed: the sim emits Toss/Feed before any live BodyHit. */
  private trackBall(kind: number): void {
    switch (kind) {
      case EventKind.Point:
      case EventKind.ServeFault:
      case EventKind.Let:
      case EventKind.GameOver:
      case EventKind.ReturnFail:
        this.ballDead = true;
        break;
      case EventKind.Toss:
      case EventKind.Feed:
        this.ballDead = false;
        break;
    }
  }

  resetReactions(): void {
    for (const s of this.slots) s.body.reset();
  }

  /** Gold flash on player i (perfect hit). */
  flash(i: number): void {
    if (i >= 0 && i < MAX_PLAYERS) this.slots[i].flash.trigger();
  }

  update(prev: Float64Array, curr: Float64Array, alpha: number, dt: number): void {
    const n = curr[Snap.NumPlayers] | 0;
    if (n !== this.lastNumPlayers) this.recolor(n);
    this.ball.set(lerp(prev[Snap.BallX], curr[Snap.BallX], alpha), lerp(prev[Snap.BallY], curr[Snap.BallY], alpha), lerp(prev[Snap.BallZ], curr[Snap.BallZ], alpha));
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
    s.body.update(p, dt);
    p.lookAt = curr[Snap.Mode] === Mode.Lobby ? null : this.ball; // the lobby has no ball in play
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
