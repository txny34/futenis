import * as THREE from "three";
import { Character, CharacterPose, StrikePose, createCharacter } from "../../character/character";
import { IMPACT_T } from "../../character/timing";
import { buildCourt } from "../../court";
import { THEME } from "../../env/theme";
import { rand } from "./util";

// Showcase court behind the arches (outside the walkable bounds): the real
// court builder + two toy athletes in an endless decorative rally. Purely
// visual and client-side; the sim knows nothing of it. Allocation-free update.
const COURT_Z = -25;
const PLAYER_X = 7.4;
const HIT_X = 6.9;
const LANE_Z = 3.2; // ball lane spread around the court center
const CROSS_S = 1.45; // one crossing (s)
const BOUNCE_S = 0.66; // fraction of the crossing when the ball bounces
const HIT_Y = 0.45;
const PEAK = 2.3;
const REBOUND = 0.9;
const CLIP_S = 0.8;
const CROSSINGS_PER_POINT = 7;
const BALL_R = 0.11;
const BALL_BOOST = 1.6;
const STEP_SPEED = 0.45;
const FLOOR_PAD = { w: 44, d: 30 } as const;

interface Athlete { readonly ch: Character; readonly pose: CharacterPose; readonly strike: StrikePose; readonly side: 1 | -1; z: number }

function athlete(shirt: number, side: 1 | -1): Athlete {
  const ch = createCharacter({ shirt, shorts: THEME.crowdBody[0], skin: THEME.crowdSkin[side > 0 ? 1 : 0], hair: THEME.crowdBody[3], boots: THEME.lobbyDecor.neon[1] });
  const strike: StrikePose = { tier: "low", t: 0, contact: null };
  return { ch, strike, side, z: 0, pose: { speed: 0, airborne: false, verticalVel: 0, charge: 0, strike, facing: side * Math.PI / 2 } };
}

export class Showcase {
  readonly group = new THREE.Group();
  private readonly ball: THREE.Mesh;
  private readonly players: readonly [Athlete, Athlete];
  private time = 0;
  private crossing = -1;
  private score: [number, number] = [0, 0];

  constructor(private readonly onScore: (a: number, b: number) => void) {
    const court = buildCourt();
    court.remove(court.children[0]); // apron: the lobby has its own floor
    const ground = new THREE.Mesh(new THREE.PlaneGeometry(FLOOR_PAD.w, FLOOR_PAD.d), new THREE.MeshStandardMaterial({ color: THEME.apron, roughness: 0.8 }));
    ground.rotation.x = -Math.PI / 2;
    ground.position.set(0, -0.005, 0);
    this.group.add(ground, court);
    this.ball = new THREE.Mesh(new THREE.SphereGeometry(BALL_R, 16, 12), new THREE.MeshBasicMaterial({ color: new THREE.Color(THEME.ball).multiplyScalar(BALL_BOOST) }));
    this.players = [athlete(THEME.team[0], 1), athlete(THEME.team[1], -1)];
    for (const p of this.players) this.group.add(p.ch.root);
    this.group.add(this.ball);
    this.group.position.z = COURT_Z;
  }

  update(dt: number): void {
    if (!(dt > 0)) return; // dt = 0 would put NaN into pose.speed -> NaN bones -> black bloom blocks
    this.time += dt;
    const k = Math.floor(this.time / CROSS_S);
    const s = this.time / CROSS_S - k;
    if (k !== this.crossing) this.newCrossing(k);
    const dir = k % 2 === 0 ? 1 : -1; // +1: player 0 (-X) hits toward +X
    const z0 = this.laneZ(k - 1);
    const z1 = this.laneZ(k);
    this.ball.position.set(-dir * HIT_X + dir * 2 * HIT_X * s, this.ballY(s), z0 + (z1 - z0) * s);
    this.ball.rotation.z -= dir * dt * 12;
    for (let i = 0; i < 2; i++) this.updateAthlete(this.players[i], i, k, s, dt);
  }

  private ballY(s: number): number {
    if (s < BOUNCE_S) { const u = s / BOUNCE_S; return HIT_Y * (1 - u) + 4 * PEAK * u * (1 - u) + BALL_R; }
    const u = (s - BOUNCE_S) / (1 - BOUNCE_S);
    return HIT_Y * u + 4 * REBOUND * u * (1 - u) + BALL_R;
  }

  private laneZ(k: number): number { return (rand(k * 0.731) * 2 - 1) * LANE_Z; }

  private newCrossing(k: number): void {
    this.crossing = k;
    if (k > 0 && k % CROSSINGS_PER_POINT === 0) {
      this.score[k % 2]++;
      this.onScore(this.score[0], this.score[1]);
    }
  }

  /** Player i hits at the start of crossings with parity i; walks toward the next lane in between. */
  private updateAthlete(a: Athlete, i: number, k: number, s: number, dt: number): void {
    const receiving = (k % 2) !== i;
    const target = receiving ? this.laneZ(k) : this.laneZ(k - 1);
    const dz = target - a.z;
    const step = Math.sign(dz) * Math.min(Math.abs(dz), STEP_SPEED * 6 * dt);
    a.z += step;
    a.pose.speed = Math.min(1, Math.abs(step) / dt / 6);
    const sinceHit = receiving ? (s - 1) * CROSS_S : s * CROSS_S; // time relative to own hit
    const t = IMPACT_T + sinceHit / CLIP_S;
    a.strike.t = t > 0 && t < 1 ? t : 0;
    a.ch.root.position.set(-a.side * PLAYER_X, 0, a.z);
    a.ch.update(a.pose, dt);
  }
}
