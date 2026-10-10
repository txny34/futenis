import * as THREE from "three";
import { LedBoard } from "./led-board";
import { Neon } from "./neon";
import { buildFog, buildSky } from "./sky";
import { Stands, buildStands } from "./stands";
import { buildStructure } from "./structure";

// Stadium around the court: sky dome, fog, tribunes + crowd, roof/towers,
// neon shapes and the LED strip. Purely cosmetic; the sim knows nothing of it.
const HYPE_RISE_S = 0.25; // ease toward a higher target (cheer kicks in fast)
const HYPE_DECAY_PER_S = 0.35; // target falls back to calm
const HYPE_SPEEDUP = 1; // bob runs (1 + hype x this) times faster

export class Arena {
  private readonly stands: Stands;
  private hypeTarget = 0;
  private readonly neon = new Neon();
  private readonly led = new LedBoard();

  constructor(scene: THREE.Scene) {
    scene.fog = buildFog();
    this.stands = buildStands();
    scene.add(buildSky(), this.stands.group, buildStructure(), this.neon.group, this.led.group);
  }

  /** Lobby (lobby.md §7): the walkable area reaches the tribunes, so the crowd and LED strip hide. */
  setLobby(on: boolean): void {
    this.stands.group.visible = !on;
    this.led.group.visible = !on;
  }

  /** Crowd excitement 0..1; the strongest recent cheer wins, then it calms down. */
  cheer(level: number): void {
    this.hypeTarget = Math.max(this.hypeTarget, Math.min(1, level));
  }

  /** Allocation-free: scalar uniforms, two material colors, three texture offsets. */
  update(dt: number): void {
    const h = this.stands.hype;
    this.hypeTarget = Math.max(0, this.hypeTarget - HYPE_DECAY_PER_S * dt);
    h.value += (this.hypeTarget - h.value) * (1 - Math.exp(-dt / HYPE_RISE_S));
    this.stands.time.value += dt * (1 + h.value * HYPE_SPEEDUP);
    this.neon.update(dt);
    this.led.update(dt);
  }
}
