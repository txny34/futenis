import * as THREE from "three";
import { CharacterPose } from "./animator";
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
const STRIKE_REACH = 0.45;
const STRIKE_SIDE_X = -0.12; // right-foot side in character space
const BICYCLE_ROOT_Y = 0.75;
const FACING = Math.PI / 2; // faces +X like player 0

export interface DemoFrame {
  readonly pose: CharacterPose;
  rootY: number;
  ballVisible: boolean;
  readonly ball: THREE.Vector3;
}

const UP = new THREE.Vector3(0, 1, 0);

export function createDriver(): { frame: DemoFrame; step(state: DemoState, speed: number, t: number): void } {
  const contact = new THREE.Vector3();
  const pose: CharacterPose = {
    speed: 0, sprinting: false, airborne: false, verticalVel: 0, charge: 0, strike: null, facing: FACING,
  };
  const strike = { part: "foot" as StrikePart, t: 0, contact: contact as THREE.Vector3 | null };
  const frame: DemoFrame = { pose, rootY: 0, ballVisible: false, ball: contact };

  function jump(time: number): void {
    const air = (2 * JUMP_V0) / GRAVITY;
    const u = time % (air + JUMP_GROUND_TIME);
    pose.airborne = u < air;
    frame.rootY = pose.airborne ? JUMP_V0 * u - 0.5 * GRAVITY * u * u : 0;
    pose.verticalVel = pose.airborne ? JUMP_V0 - GRAVITY * u : 0;
  }

  function strikeDemo(part: StrikePart, time: number): void {
    const u = time % (STRIKE_TIME + STRIKE_REST);
    const swinging = u >= STRIKE_REST;
    strike.part = part;
    strike.t = swinging ? (u - STRIKE_REST) / STRIKE_TIME : 0;
    pose.charge = swinging ? 0 : u / STRIKE_REST;
    pose.strike = strike;
    const bicycle = part === "bicycle";
    pose.airborne = bicycle;
    frame.rootY = bicycle ? BICYCLE_ROOT_Y : 0;
    const ballY = bicycle ? SIM_ZONES.bicycle.ideal : SIM_ZONES[part].ideal;
    contact.set(STRIKE_SIDE_X, ballY, STRIKE_REACH).applyAxisAngle(UP, FACING);
    frame.ballVisible = true;
  }

  function step(state: DemoState, speed: number, time: number): void {
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
      default: strikeDemo(state, time);
    }
  }

  return { frame, step };
}
