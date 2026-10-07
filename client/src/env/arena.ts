import * as THREE from "three";
import { LedBoard } from "./led-board";
import { Neon } from "./neon";
import { buildFog, buildSky } from "./sky";
import { Stands, buildStands } from "./stands";
import { buildStructure } from "./structure";

// Stadium around the court: sky dome, fog, tribunes + crowd, roof/towers,
// neon shapes and the LED strip. Purely cosmetic; the sim knows nothing of it.
export class Arena {
  private readonly stands: Stands;
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

  /** Allocation-free: scalar uniform, two material colors, three texture offsets. */
  update(dt: number): void {
    this.stands.time.value += dt;
    this.neon.update(dt);
    this.led.update(dt);
  }
}
