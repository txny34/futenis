import * as THREE from "three";
import { BODY, SIM_ZONES, StrikePart } from "./config";

// Validation overlay: hitbox capsule + one horizontal ring per sim zone
// height (ideal solid, min/max faint). Parent to the character's world position, not to a bone.

const CAPSULE_COLOR = 0x39d353;
const CAPSULE_OPACITY = 0.18;
const RING_GAP = 0.03;
const RING_SEGMENTS = 32;
const FAINT_OPACITY = 0.35;
const ZONE_COLORS: Readonly<Record<StrikePart, number>> = {
  foot: 0x58a6ff, knee: 0x3fb950, chest: 0xd29922,
  shoulder: 0xdb61a2, head: 0xf85149, bicycle: 0xa371f7,
};

function ring(y: number, radius: number, color: number, opacity: number): THREE.LineLoop {
  const pts: THREE.Vector3[] = [];
  for (let i = 0; i < RING_SEGMENTS; i++) {
    const a = (i / RING_SEGMENTS) * Math.PI * 2;
    pts.push(new THREE.Vector3(Math.cos(a) * radius, y, Math.sin(a) * radius));
  }
  const geo = new THREE.BufferGeometry().setFromPoints(pts);
  const mat = new THREE.LineBasicMaterial({ color, transparent: opacity < 1, opacity });
  return new THREE.LineLoop(geo, mat);
}

export function createHitboxOverlay(): THREE.Group {
  const group = new THREE.Group();
  const r = BODY.capsuleRadius;
  const capsule = new THREE.Mesh(
    new THREE.CapsuleGeometry(r, BODY.height - 2 * r, 6, 16),
    new THREE.MeshBasicMaterial({
      color: CAPSULE_COLOR, wireframe: true, transparent: true, opacity: CAPSULE_OPACITY,
    }),
  );
  capsule.position.y = BODY.height / 2;
  group.add(capsule);
  const parts = Object.keys(SIM_ZONES) as StrikePart[];
  parts.forEach((part, i) => {
    const radius = r + RING_GAP * (1 + (i % 2));
    const band = SIM_ZONES[part];
    group.add(ring(band.ideal, radius, ZONE_COLORS[part], 1));
    group.add(ring(band.min, radius, ZONE_COLORS[part], FAINT_OPACITY));
    group.add(ring(band.max, radius, ZONE_COLORS[part], FAINT_OPACITY));
  });
  return group;
}

