import * as THREE from "three";
import { LOBBY_PAD_COUNT, padX, padZ } from "../pads";
import { Ambient } from "./ambient";
import { Arches } from "./arches";
import { Confetti } from "./confetti";
import { LobbyFloor, buildFloor } from "./floor";
import { Npcs } from "./npcs";
import { PadFx } from "./pad-fx";
import { buildPosters } from "./posters";
import { Scoreboard } from "./screen";
import { Showcase } from "./showcase";
import { HallOfFame } from "../zone2/hall";

// Lobby decor pass (docs/design/lobby.md §7.1). Everything lives under one
// group owned by LobbyView, so setLobby(false) hides it all (lights included).
// update() is allocation-free: uniforms, matrices, a throttled canvas redraw.
const CONFETTI_START = 0.7; // calm burst, pad start only

export class LobbyDecor {
  readonly group = new THREE.Group();
  /** Always in the scene; dimmed to 0 outside the lobby so material programs never recompile. */
  readonly lights = new THREE.Group();
  private readonly lit: Array<readonly [THREE.Light, number]> = [];
  private readonly floor: LobbyFloor;
  private readonly arches = new Arches();
  private readonly screen = new Scoreboard();
  private readonly pads = new PadFx();
  private readonly ambient = new Ambient();
  private readonly confetti = new Confetti();
  private readonly npcs = new Npcs();
  private readonly showcase: Showcase;
  private readonly hall = new HallOfFame();
  private time = 0;

  constructor(halfX: number, halfZ: number) {
    this.floor = buildFloor(halfX, halfZ);
    this.showcase = new Showcase((a, b) => this.screen.setScore(a, b));
    this.group.add(this.floor.mesh, this.arches.group, this.screen.group, buildPosters(), this.pads.group,
      this.ambient.group, this.confetti.points, this.npcs.group, this.showcase.group, this.showcase.stage, this.hall.group);
    this.lights.add(this.ambient.lights, this.showcase.lights, this.hall.lights);
    this.lights.traverse((o) => { if (o instanceof THREE.Light) this.lit.push([o, o.intensity]); });
    this.setLit(false);
  }

  /** Lobby lights on (base intensity) or off (0). Not per frame. */
  setLit(on: boolean): void {
    for (const [l, base] of this.lit) l.intensity = on ? base : 0;
  }

  /** Pad i heat 0..1 (hold progress, pulse, flash). */
  setPadHeat(i: number, h: number): void { this.pads.setHeat(i, h); }

  /** Calm confetti at pad start (EvPadStart only). */
  burst(pad: number): void {
    if (pad < 0 || pad >= LOBBY_PAD_COUNT) return;
    this.confetti.burst(padX(pad), padZ(pad), CONFETTI_START);
  }

  update(dt: number): void {
    this.time += dt;
    this.floor.update(this.time);
    this.screen.update(dt);
    this.pads.update(this.time);
    this.ambient.update(this.time);
    this.confetti.update(dt);
    this.npcs.update(dt);
    this.showcase.update(dt);
  }
}
