import * as THREE from "three";
import { BODY, SIM_ZONES, StrikePart } from "./config";

// Validation overlay: hitbox capsule + sim zone height lines (ideal solid,
// min/max faint). Parent to the character's world position, not to a bone.

const CAPSULE_COLOR = 0x39d353;
const CAPSULE_OPACITY = 0.18;
const LINE_HALF = 0.55;
const FAINT_OPACITY = 0.35;
const ZONE_COLORS: Readonly<Record<StrikePart, number>> = {
  foot: 0x58a6ff, knee: 0x3fb950, chest: 0xd29922,
  shoulder: 0xdb61a2, head: 0xf85149, bicycle: 0xa371f7,
};

function line(y: number, color: number, opacity: number, z: number): THREE.Line {
  const geo = new THREE.BufferGeometry().setFromPoints([
    new THREE.Vector3(-LINE_HALF, y, z),
    new THREE.Vector3(LINE_HALF, y, z),
  ]);
  const mat = new THREE.LineBasicMaterial({ color, transparent: opacity < 1, opacity });
  return new THREE.Line(geo, mat);
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
    const z = r + 0.02 * i;
    const band = SIM_ZONES[part];
    group.add(line(band.ideal, ZONE_COLORS[part], 1, z));
    group.add(line(band.min, ZONE_COLORS[part], FAINT_OPACITY, z));
    group.add(line(band.max, ZONE_COLORS[part], FAINT_OPACITY, z));
  });
  return group;
}

