import * as THREE from "three";
import { CharacterPose, StrikePose } from "./animator";
import { SIM_ZONES, StrikePart } from "./config";

// Drives a CharacterPose for the preview from a selected demo state. Writes
// into one preallocated pose / contact vector each frame.

export type DemoState = "idle" | "run" | "sprint" | "jump" | "charge" | StrikePart;

export const DEMO_STATES: readonly DemoState[] = [
  "idle", "run", "sprint", "jump", "charge", "foot", "knee", "chest", "shoulder", "head", "bicycle",
];

const GRAVITY = 9.81;
const JUMP_V0 = 4.5;
const JUMP_GROUND_TIME = 0.35;
const CHARGE_PERIOD = 1.6;
const STRIKE_TIME = 0.9;
const STRIKE_REST = 0.9;
const BICYCLE_APEX = 0.25; // root apex; hips peak ~1.24 m
const FACING = Math.PI / 2; // faces +X like player 0

export type SideMode = "R" | "L" | "auto";

// Demo ball in character space: |x| toward the strike side (right = -X),
// y at the sim zone ideal (bicycle: world height of a scissor volley), z ahead.
const DEMO_BALL: Readonly<Record<StrikePart, readonly [number, number, number]>> = {
  foot: [0.15, SIM_ZONES.foot.ideal, 0.45],
  knee: [0.12, SIM_ZONES.knee.ideal, 0.35],
  chest: [0, SIM_ZONES.chest.ideal, 0.3],
  shoulder: [0.22, SIM_ZONES.shoulder.ideal, 0.15],
  head: [0, SIM_ZONES.head.ideal, 0.2],
  bicycle: [0.1, 1.95, 0.15],
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
    speed: 0, sprinting: false, airborne: false, verticalVel: 0, charge: 0, strike: null, facing: FACING,
  };
  const strike: StrikePose = { part: "foot", t: 0, contact };
  const frame: DemoFrame = { pose, rootY: 0, ballVisible: false, ball: contact };

  function jump(time: number): void {
    const air = (2 * JUMP_V0) / GRAVITY;
    const u = time % (air + JUMP_GROUND_TIME);
    pose.airborne = u < air;
    frame.rootY = pose.airborne ? JUMP_V0 * u - 0.5 * GRAVITY * u * u : 0;
    pose.verticalVel = pose.airborne ? JUMP_V0 - GRAVITY * u : 0;
  }

  function strikeDemo(part: StrikePart, time: number, scrub: number | null, mode: SideMode): void {
    const cycle = STRIKE_TIME + STRIKE_REST;
    const u = time % cycle;
    const swinging = scrub !== null || u >= STRIKE_REST;
    strike.part = part;
    strike.t = scrub !== null ? scrub : swinging ? (u - STRIKE_REST) / STRIKE_TIME : 0;
    pose.charge = swinging ? (strike.t === 0 ? 1 : 0) : u / STRIKE_REST;
    pose.strike = strike;
    const left = mode === "L" || (mode === "auto" && Math.floor(time / cycle) % 2 === 1);
    strike.side = mode === "auto" ? undefined : mode;
    const bicycle = part === "bicycle" && strike.t > 0;
    pose.airborne = bicycle;
    frame.rootY = bicycle ? BICYCLE_APEX * Math.sin(Math.PI * strike.t) : 0;
    const [bx, by, bz] = DEMO_BALL[part];
    contact.set(left ? bx : -bx, by, bz).applyAxisAngle(UP, FACING);
    frame.ballVisible = true;
  }

  function step(state: DemoState, speed: number, time: number, scrub: number | null, side: SideMode): void {
    pose.speed = 0; pose.sprinting = false; pose.airborne = false;
    pose.verticalVel = 0; pose.charge = 0; pose.strike = null;
    frame.rootY = 0; frame.ballVisible = false;
    switch (state) {
      case "idle": return;
      case "run": pose.speed = speed; return;
      case "sprint": pose.speed = 1; pose.sprinting = true; return;
      case "jump": jump(time); return;
      case "charge":
        pose.charge = 1 - Math.abs(((time / CHARGE_PERIOD) % 1) * 2 - 1);
        return;
      default: strikeDemo(state, time, scrub, side);
    }
  }

  return { frame, step };
}
