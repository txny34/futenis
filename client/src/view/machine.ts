import * as THREE from "three";
import { THEME } from "../env/theme";
import { Mode, Snap } from "../snapshot";

// Entrenamiento ball machine (docs/design/training.md §5): procedural box +
// barrel on the floor at SnapMachineX/Z, barrel pointing at the player's half
// (-X). Glides to each new mouth position and flashes on EvFeed. Purely
// visual: the sim owns the launch. Allocation-free per frame.
const BODY_W = 0.7; // along Z
const BODY_D = 0.6; // along X
const BODY_H = 0.75;
const LEG_H = 0.12;
const BARREL_R = 0.13;
const BARREL_L = 0.55;
const BARREL_TILT = 0.35; // rad up from horizontal
const MOUTH_Y = 1.0; // sim launches from Y = 1.0 m: the barrel tip sits near it
const GLIDE_RATE = 10; // 1/s exponential approach to a new position
const FLASH_S = 0.35;
const FLASH_GAIN = 1.6;
const TRIM_EMISSIVE = 0.5;

export class MachineView {
  private readonly root = new THREE.Group();
  private readonly body: THREE.MeshStandardMaterial;
  private readonly trim: THREE.MeshStandardMaterial;
  private readonly flashColor = new THREE.Color(THEME.machine.flash);
  private flashLeft = 0;
  private placed = false;

  constructor(scene: THREE.Scene) {
    this.body = new THREE.MeshStandardMaterial({ color: THEME.machine.body, roughness: 0.55, metalness: 0.3 });
    this.trim = new THREE.MeshStandardMaterial({ color: THEME.machine.trim, emissive: THEME.machine.trim, emissiveIntensity: TRIM_EMISSIVE });
    const barrelMat = new THREE.MeshStandardMaterial({ color: THEME.machine.barrel, roughness: 0.4, metalness: 0.6 });

    const box = new THREE.Mesh(new THREE.BoxGeometry(BODY_D, BODY_H, BODY_W), this.body);
    box.position.y = LEG_H + BODY_H / 2;
    const band = new THREE.Mesh(new THREE.BoxGeometry(BODY_D + 0.02, 0.06, BODY_W + 0.02), this.trim);
    band.position.y = LEG_H + BODY_H * 0.8;
    const base = new THREE.Mesh(new THREE.BoxGeometry(BODY_D * 1.2, LEG_H, BODY_W * 1.2), barrelMat);
    base.position.y = LEG_H / 2;

    // Barrel: cylinder along local Y, rotated to point -X and tilted up.
    const barrel = new THREE.Mesh(new THREE.CylinderGeometry(BARREL_R, BARREL_R * 1.15, BARREL_L, 16), barrelMat);
    barrel.rotation.z = Math.PI / 2 - BARREL_TILT;
    barrel.position.set(-BODY_D / 2 - Math.cos(BARREL_TILT) * BARREL_L * 0.35, MOUTH_Y - Math.sin(BARREL_TILT) * BARREL_L * 0.5, 0);
    const ring = new THREE.Mesh(new THREE.TorusGeometry(BARREL_R * 1.15, 0.025, 8, 20), this.trim);
    ring.rotation.y = Math.PI / 2;
    ring.rotation.x = BARREL_TILT;
    ring.position.set(barrel.position.x - Math.cos(BARREL_TILT) * BARREL_L / 2, barrel.position.y + Math.sin(BARREL_TILT) * BARREL_L / 2, 0);

    for (const m of [box, band, base, barrel]) m.castShadow = true;
    this.root.add(box, band, base, barrel, ring);
    this.root.visible = false;
    scene.add(this.root);
  }

  /** EvFeed: the machine just launched. */
  flash(): void { this.flashLeft = FLASH_S; }

  update(curr: Float64Array, dt: number): void {
    const x = curr[Snap.MachineX];
    const on = curr[Snap.Mode] === Mode.Playground && x > 0; // X = 0 before the first feed is sampled
    this.root.visible = on;
    if (!on) {
      this.placed = false;
      return;
    }
    const z = curr[Snap.MachineZ];
    const p = this.root.position;
    if (!this.placed || dt <= 0) {
      if (!this.placed) p.set(x, 0, z);
      this.placed = true;
    } else {
      const k = 1 - Math.exp(-GLIDE_RATE * dt);
      p.x += (x - p.x) * k;
      p.z += (z - p.z) * k;
    }
    this.updateFlash(dt);
  }

  private updateFlash(dt: number): void {
    if (this.flashLeft <= 0) return;
    this.flashLeft = Math.max(0, this.flashLeft - dt);
    const k = (this.flashLeft / FLASH_S) * FLASH_GAIN;
    const c = this.flashColor;
    this.body.emissive.setRGB(c.r * k, c.g * k, c.b * k);
    this.trim.emissiveIntensity = TRIM_EMISSIVE + k;
  }
}
