import * as THREE from "three";
import { THEME } from "../env/theme";
import { Quality } from "../snapshot";

// osu!-style approach circle around the human (contact-v2 §22.3): while
// PredictContact says "standing still, you'd reach the ball in N ticks", a
// ring closes from APPROACH_R down to the body (TARGET_R) as N -> 0. Color is
// the predicted quality. Allocation-free per frame.
const ORDER = 12;
const FLOOR_Y = 0.025;
const TARGET_R = 0.42; // ~body capsule: the ring "lands" here at contact
const APPROACH_R = 2.1;
const CLOSE_TICKS = 72; // 0.6 s at 120 Hz: farther predictions sit at APPROACH_R
const RING_INNER = 0.9; // unit ring, scaled to the radius
const TARGET_INNER = 0.36;
const TARGET_OUTER = 0.42;
const SEGMENTS = 64;
const TARGET_OPACITY = 0.35;
const RING_OPACITY = 0.9;
const MIN_OPACITY = 0.35;
const QUALITY_COLORS: readonly number[] = THEME.fx.quality; // perfect / good / forced

function floorMesh(geo: THREE.BufferGeometry, color: number, opacity: number): THREE.Mesh<THREE.BufferGeometry, THREE.MeshBasicMaterial> {
  const mat = new THREE.MeshBasicMaterial({ color, transparent: true, opacity, depthTest: false, depthWrite: false });
  const m = new THREE.Mesh(geo, mat);
  m.rotation.x = -Math.PI / 2;
  m.renderOrder = ORDER;
  m.visible = false;
  return m;
}

export class TimingRing {
  private readonly ring = floorMesh(new THREE.RingGeometry(RING_INNER, 1, SEGMENTS), QUALITY_COLORS[Quality.Good], RING_OPACITY);
  private readonly target = floorMesh(new THREE.RingGeometry(TARGET_INNER, TARGET_OUTER, SEGMENTS), THEME.fx.ringTarget, TARGET_OPACITY);
  private lastQuality = -1;

  constructor(scene: THREE.Scene) {
    scene.add(this.target, this.ring);
  }

  hide(): void {
    this.ring.visible = false;
    this.target.visible = false;
  }

  /** ticks: predicted ticks to contact (fractional, already interpolated). */
  update(feet: THREE.Vector3, ticks: number, quality: number): void {
    const k = Math.max(0, Math.min(1, ticks / CLOSE_TICKS));
    const r = TARGET_R + (APPROACH_R - TARGET_R) * k;
    this.ring.position.set(feet.x, FLOOR_Y, feet.z);
    this.ring.scale.set(r, r, 1);
    this.ring.material.opacity = MIN_OPACITY + (RING_OPACITY - MIN_OPACITY) * (1 - k);
    this.target.position.set(feet.x, FLOOR_Y, feet.z);
    if (quality !== this.lastQuality) {
      this.lastQuality = quality;
      this.ring.material.color.setHex(QUALITY_COLORS[quality] ?? QUALITY_COLORS[Quality.Forced]);
    }
    this.ring.visible = true;
    this.target.visible = true;
  }
}
