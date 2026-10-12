import * as THREE from "three";
import { CharacterPose, StrikePose } from "./animator";
import { SIM_ZONES, StrikeTier } from "./config";

// Drives a CharacterPose for the preview from a selected demo state. Writes
// into one preallocated pose / contact vector each frame.

export type DemoState = "idle" | "run" | "jump" | "charge" | "perfect" | "cheer" | "slump" | "shuffle" | StrikeTier;

export const DEMO_STATES: readonly DemoState[] = [
  "idle", "run", "jump", "charge", "low", "mid", "high", "perfect", "cheer", "slump", "shuffle",
];

const GRAVITY = 9.81;
const JUMP_V0 = 4.5;
const JUMP_GROUND_TIME = 0.35;
const CHARGE_PERIOD = 1.6;
const STRIKE_TIME = 0.9;
const STRIKE_REST = 0.9;
const FACING = Math.PI / 2; // faces +X like player 0

export type SideMode = "R" | "L" | "auto";

// Demo ball in character space: |x| toward the strike side (right = -X),
// y at a representative sim zone ideal for the tier, z ahead.
const DEMO_BALL: Readonly<Record<StrikeTier, readonly [number, number, number]>> = {
  low: [0.15, SIM_ZONES.foot.ideal, 0.45],
  mid: [0, SIM_ZONES.chest.ideal, 0.3],
  high: [0, SIM_ZONES.head.ideal, 0.2],
};
export interface DemoFrame {
  readonly pose: CharacterPose;
  rootY: number;
  ballVisible: boolean;
  readonly ball: THREE.Vector3;
}

const UP = new THREE.Vector3(0, 1, 0);

export function createDriver(): { frame: DemoFrame; step(state: DemoState, speed: number, t: number, scrub: number | null, side: SideMode): void } {
  const contact = new THREE.Vector3();
  const pose: CharacterPose = {
    speed: 0, airborne: false, verticalVel: 0, charge: 0, strike: null, facing: FACING,
  };
  const strike: StrikePose = { tier: "low", t: 0, contact };
  const frame: DemoFrame = { pose, rootY: 0, ballVisible: false, ball: contact };

  function jump(time: number): void {
    const air = (2 * JUMP_V0) / GRAVITY;
    const u = time % (air + JUMP_GROUND_TIME);
    pose.airborne = u < air;
    frame.rootY = pose.airborne ? JUMP_V0 * u - 0.5 * GRAVITY * u * u : 0;
    pose.verticalVel = pose.airborne ? JUMP_V0 - GRAVITY * u : 0;
  }

  function strikeDemo(tier: StrikeTier, perfect: boolean, time: number, scrub: number | null, mode: SideMode): void {
    const cycle = STRIKE_TIME + STRIKE_REST;
    const u = time % cycle;
    const swinging = scrub !== null || u >= STRIKE_REST;
    strike.tier = tier;
    strike.perfect = perfect;
    strike.t = scrub !== null ? scrub : swinging ? (u - STRIKE_REST) / STRIKE_TIME : 0;
    pose.charge = swinging ? (strike.t === 0 ? 1 : 0) : u / STRIKE_REST;
    pose.strike = strike;
    const left = mode === "L" || (mode === "auto" && Math.floor(time / cycle) % 2 === 1);
    strike.side = mode === "auto" ? undefined : mode;
    const [bx, by, bz] = DEMO_BALL[tier];
    contact.set(left ? bx : -bx, by, bz).applyAxisAngle(UP, FACING);
    frame.ballVisible = true;
  }

  function step(state: DemoState, speed: number, time: number, scrub: number | null, side: SideMode): void {
    pose.speed = 0; pose.airborne = false;
    pose.verticalVel = 0; pose.charge = 0; pose.strike = null; pose.cheer = 0; pose.slump = 0; pose.strafe = 0;
    frame.rootY = 0; frame.ballVisible = false;
    switch (state) {
      case "idle": return;
      case "run": pose.speed = speed; return;
      case "cheer": pose.cheer = 1; return;
      case "shuffle": pose.speed = 0.45; pose.strafe = Math.sin(time * 0.8) > 0 ? 1 : -1; return;
      case "slump": pose.slump = 1; return;
      case "jump": jump(time); return;
      case "charge":
        pose.charge = 1 - Math.abs(((time / CHARGE_PERIOD) % 1) * 2 - 1);
        return;
      case "perfect": strikeDemo("low", true, time, scrub, side); return;
      default: strikeDemo(state, false, time, scrub, side);
    }
  }

  return { frame, step };
}
