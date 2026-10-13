import * as THREE from "three";
import { Character, CharacterPose, CharacterStyle, DEFAULT_LOOK, createCharacter } from "../../character/character";
import { BODY } from "../../character/config";
import { THEME } from "../../env/theme";
import { Turntable } from "./turntable";

// Live 3D preview for the Estilos panel: its own small renderer, the whole
// character head to sneakers, spun 360° by dragging (turntable.ts) and
// smiling now and then. Runs only while the panel is open (start/stop).

const W = 340;
const H = 560;
const SMILE_EVERY_S = 4;

export class StylePreview {
  readonly canvas: HTMLCanvasElement;
  private readonly renderer: THREE.WebGLRenderer;
  private readonly scene = new THREE.Scene();
  private readonly camera = new THREE.PerspectiveCamera(24, W / H, 0.1, 20);
  private readonly turntable: Turntable;
  private readonly character: Character;
  private readonly pose: CharacterPose = { speed: 0, airborne: false, verticalVel: 0, charge: 0, strike: null, facing: 0 };
  private raf = 0;
  private last = 0;
  private clock = 0;
  private number = -1;

  constructor() {
    this.renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
    this.renderer.setPixelRatio(Math.min(2, window.devicePixelRatio));
    this.renderer.setSize(W, H, false);
    this.canvas = this.renderer.domElement;
    this.canvas.className = "fx-styles__canvas";
    this.scene.add(new THREE.HemisphereLight(THEME.lightKey, THEME.crowdBody[1], 1.6));
    const key = new THREE.DirectionalLight(THEME.lightKey, 1.6);
    key.position.set(1.5, 3, 2.5);
    this.scene.add(key);
    this.character = createCharacter(DEFAULT_LOOK);
    this.scene.add(this.character.root);
    this.camera.position.set(0, BODY.height * 0.52, 4.9); // the whole body, feet included
    this.camera.lookAt(0, BODY.height * 0.49, 0);
    this.turntable = new Turntable(this.canvas);
  }

  setStyle(style: Readonly<CharacterStyle>): void {
    this.character.setStyle(style);
    this.character.setJersey(style.number, "");
    if (this.number >= 0 && style.number !== this.number) this.turntable.turnTo(Math.PI); // show the number
    this.number = style.number;
    this.character.express("happy", 1.2);
  }

  start(): void {
    if (this.raf) return;
    this.last = performance.now();
    const frame = (now: number): void => {
      const dt = Math.min(0.05, (now - this.last) / 1000);
      this.last = now;
      this.clock += dt;
      this.turntable.update(dt);
      this.pose.facing = this.turntable.yaw;
      if (Math.floor(this.clock / SMILE_EVERY_S) !== Math.floor((this.clock - dt) / SMILE_EVERY_S)) this.character.express("cheer", 1);
      this.character.update(this.pose, dt);
      this.renderer.render(this.scene, this.camera);
      this.raf = requestAnimationFrame(frame);
    };
    this.raf = requestAnimationFrame(frame);
  }

  stop(): void {
    cancelAnimationFrame(this.raf);
    this.raf = 0;
  }
}
