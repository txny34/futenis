import * as THREE from "three";
import { THEME } from "../env/theme";

// Gold emissive flash on a character after a perfect hit. Collects the
// character's emissive-capable materials once (construction), then only
// scales one shared color per frame: no allocations.
const FLASH_S = 0.5;
const FLASH_COLOR = new THREE.Color(THEME.fx.perfectFlash);
const FLASH_GAIN = 0.9;

function hasEmissive(m: THREE.Material): m is THREE.MeshToonMaterial | THREE.MeshStandardMaterial {
  return m instanceof THREE.MeshToonMaterial || m instanceof THREE.MeshStandardMaterial;
}

export class HitFlash {
  private readonly mats: (THREE.MeshToonMaterial | THREE.MeshStandardMaterial)[] = [];
  private left = 0;

  constructor(root: THREE.Object3D) {
    const seen = new Set<THREE.Material>();
    root.traverse((o) => {
      if (!(o instanceof THREE.Mesh)) return;
      const list: THREE.Material[] = Array.isArray(o.material) ? o.material : [o.material];
      for (const m of list) {
        if (!seen.has(m) && hasEmissive(m)) this.mats.push(m);
        seen.add(m);
      }
    });
  }

  trigger(): void { this.left = FLASH_S; }

  update(dt: number): void {
    if (this.left <= 0) return;
    this.left = Math.max(0, this.left - dt);
    const k = (this.left / FLASH_S) * FLASH_GAIN;
    for (const m of this.mats) m.emissive.setRGB(FLASH_COLOR.r * k, FLASH_COLOR.g * k, FLASH_COLOR.b * k);
  }
}
