import * as THREE from "three";
import { EffectComposer } from "three/addons/postprocessing/EffectComposer.js";
import { OutputPass } from "three/addons/postprocessing/OutputPass.js";
import { RenderPass } from "three/addons/postprocessing/RenderPass.js";
import { UnrealBloomPass } from "three/addons/postprocessing/UnrealBloomPass.js";
import type { Quality } from "../ui/settings";

// Bloom pipeline: RenderPass -> UnrealBloomPass -> OutputPass (ACES tone
// mapping + sRGB happen in OutputPass). Only pixels above the threshold glow:
// neon tubes, lamp heads, LED strip, court lines and the ball are pushed > 1.
// "low" skips the composer entirely and renders straight to the canvas
// (the renderer then applies tone mapping + sRGB itself).
const BLOOM_THRESHOLD = 0.85;
const BLOOM_RADIUS = 0.45;
const BLOOM_STRENGTH: Readonly<Record<Quality, number>> = { low: 0, med: 0.7, high: 0.85 };
// Lobby (calm night arena, photosensitivity): ~45% of the match bloom, higher threshold.
const LOBBY_BLOOM_SCALE = 0.45;
const LOBBY_BLOOM_THRESHOLD = 1.0;
const MSAA_SAMPLES = 4; // composer targets lose the canvas MSAA; restore it here
// Bloom mips are blurred anyway: render them at a fraction of the canvas.
const BLOOM_RES_SCALE: Readonly<Record<Quality, number>> = { low: 0, med: 0.5, high: 0.75 };

export class PostFx {
  private readonly composer: EffectComposer;
  private readonly bloom: UnrealBloomPass;
  private enabled = true;
  private resScale = BLOOM_RES_SCALE.high;
  private quality: Quality = "high";
  private lobby = false;

  constructor(
    private readonly renderer: THREE.WebGLRenderer,
    private readonly scene: THREE.Scene,
    private readonly camera: THREE.Camera,
  ) {
    renderer.toneMapping = THREE.ACESFilmicToneMapping;
    renderer.outputColorSpace = THREE.SRGBColorSpace;
    const target = new THREE.WebGLRenderTarget(1, 1, { type: THREE.HalfFloatType, samples: MSAA_SAMPLES });
    this.composer = new EffectComposer(renderer, target);
    this.composer.addPass(new RenderPass(scene, camera));
    this.bloom = new UnrealBloomPass(new THREE.Vector2(1, 1), BLOOM_STRENGTH.high, BLOOM_RADIUS, BLOOM_THRESHOLD);
    this.composer.addPass(this.bloom);
    this.composer.addPass(new OutputPass());
  }

  setQuality(q: Quality): void {
    this.quality = q;
    this.enabled = BLOOM_STRENGTH[q] > 0;
    this.resScale = BLOOM_RES_SCALE[q];
    this.applyBloom();
  }

  /** Lobby gets a softer bloom; matches restore the normal one. Not per frame. */
  setLobby(on: boolean): void {
    this.lobby = on;
    this.applyBloom();
  }

  private applyBloom(): void {
    const s = BLOOM_STRENGTH[this.quality];
    this.bloom.strength = this.lobby ? s * LOBBY_BLOOM_SCALE : s;
    this.bloom.threshold = this.lobby ? LOBBY_BLOOM_THRESHOLD : BLOOM_THRESHOLD;
  }

  /** Call after renderer.setSize / setPixelRatio. */
  resize(w: number, h: number): void {
    this.composer.setPixelRatio(this.renderer.getPixelRatio());
    this.composer.setSize(w, h);
    const pr = this.renderer.getPixelRatio();
    this.bloom.setSize(Math.max(1, Math.round(w * pr * this.resScale)), Math.max(1, Math.round(h * pr * this.resScale)));
  }

  render(dt: number): void {
    if (this.enabled) this.composer.render(dt);
    else this.renderer.render(this.scene, this.camera);
  }
}
