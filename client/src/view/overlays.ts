import * as THREE from "three";
import { THEME } from "../env/theme";

// Human-only court overlay: the aim reticle on the floor (the strike charge lives
// on the HUD crosshair, hud-charge.ts). Allocation-free per frame.
const OVERLAY_ORDER = 10;
const FLOOR_Y = 0.01;
const RETICLE_INNER = 0.28;
const RETICLE_OUTER = 0.36;
const RETICLE_COLOR = THEME.fx.reticle;
const RETICLE_OPACITY = 0.85;
const RING_SEGMENTS = 48;

function overlayMat(color: number, opacity = 1): THREE.MeshBasicMaterial {
  return new THREE.MeshBasicMaterial({ color, transparent: opacity < 1, opacity, depthTest: false, depthWrite: false });
}

function floorMesh(geo: THREE.BufferGeometry, mat: THREE.Material): THREE.Mesh {
  const m = new THREE.Mesh(geo, mat);
  m.rotation.x = -Math.PI / 2;
  m.renderOrder = OVERLAY_ORDER;
  return m;
}

export class Overlays {
  private readonly reticle: THREE.Mesh;

  constructor(scene: THREE.Scene) {
    this.reticle = floorMesh(new THREE.RingGeometry(RETICLE_INNER, RETICLE_OUTER, RING_SEGMENTS), overlayMat(RETICLE_COLOR, RETICLE_OPACITY));
    scene.add(this.reticle);
    this.setVisible(false);
  }

  setVisible(v: boolean): void {
    this.reticle.visible = v;
  }

  update(targetX: number, targetZ: number): void {
    this.reticle.position.set(targetX, FLOOR_Y, targetZ);
  }
}
