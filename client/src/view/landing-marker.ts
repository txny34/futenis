import * as THREE from "three";
import { THEME } from "../env/theme";

// Landing marker (contact-v2 §22.2): ring + dot on the floor where the ball
// will next bounce. Pulses faster as the bounce approaches. Allocation-free.
const ORDER = 11;
const FLOOR_Y = 0.02;
const RING_INNER = 0.3;
const RING_OUTER = 0.38;
const DOT_R = 0.08;
const SEGMENTS = 40;
const COLOR = THEME.fx.landing;
const RING_OPACITY = 0.8;
const NEAR_TICKS = 60; // inside this the ring shrinks toward the dot
const MIN_SCALE = 0.55;

function floorMesh(geo: THREE.BufferGeometry, opacity: number): THREE.Mesh {
  const mat = new THREE.MeshBasicMaterial({ color: COLOR, transparent: true, opacity, depthTest: false, depthWrite: false });
  const m = new THREE.Mesh(geo, mat);
  m.rotation.x = -Math.PI / 2;
  m.renderOrder = ORDER;
  return m;
}

export class LandingMarker {
  private readonly group = new THREE.Group();
  private readonly ring = floorMesh(new THREE.RingGeometry(RING_INNER, RING_OUTER, SEGMENTS), RING_OPACITY);

  constructor(scene: THREE.Scene) {
    this.group.add(this.ring, floorMesh(new THREE.CircleGeometry(DOT_R, SEGMENTS), 1));
    this.group.visible = false;
    scene.add(this.group);
  }

  hide(): void { this.group.visible = false; }

  update(x: number, z: number, ticks: number): void {
    const k = Math.max(0, Math.min(1, ticks / NEAR_TICKS));
    const s = MIN_SCALE + (1 - MIN_SCALE) * k;
    this.group.position.set(x, FLOOR_Y, z);
    this.ring.scale.set(s, s, 1);
    this.group.visible = true;
  }
}
