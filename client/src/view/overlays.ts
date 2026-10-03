import * as THREE from "three";

// Human-only court overlays: aim reticle and charge ring (on the floor). Allocation-free per frame.
const OVERLAY_ORDER = 10;
const FLOOR_Y = 0.01;
const RETICLE_INNER = 0.28;
const RETICLE_OUTER = 0.36;
const RETICLE_COLOR = 0xffffff;
const RETICLE_OPACITY = 0.85;
const RING_INNER = 0.55;
const RING_OUTER = 0.68;
const RING_SEGMENTS = 48;
const INDICES_PER_SEGMENT = 6; // RingGeometry with 1 phi segment: 2 tris per theta segment
const RING_COLOR = 0xffd23f;
const RING_START = Math.PI / 2; // charge fills clockwise from "12 o'clock" seen from above

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
  private readonly ring: THREE.Mesh;
  private readonly ringGeo: THREE.RingGeometry;

  constructor(scene: THREE.Scene) {
    this.reticle = floorMesh(new THREE.RingGeometry(RETICLE_INNER, RETICLE_OUTER, RING_SEGMENTS), overlayMat(RETICLE_COLOR, RETICLE_OPACITY));
    this.ringGeo = new THREE.RingGeometry(RING_INNER, RING_OUTER, RING_SEGMENTS, 1, RING_START, Math.PI * 2);
    this.ring = floorMesh(this.ringGeo, overlayMat(RING_COLOR));
    scene.add(this.reticle, this.ring);
    this.setVisible(false);
  }

  setVisible(v: boolean): void {
    this.reticle.visible = v;
    this.ring.visible = v;
  }

  update(feet: THREE.Vector3, targetX: number, targetZ: number, charge: number): void {
    this.reticle.position.set(targetX, FLOOR_Y, targetZ);
    this.ring.position.set(feet.x, FLOOR_Y, feet.z);
    const segs = Math.round(Math.max(0, Math.min(1, charge)) * RING_SEGMENTS);
    this.ring.visible = segs > 0;
    this.ringGeo.setDrawRange(0, segs * INDICES_PER_SEGMENT);
  }
}
