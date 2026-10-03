import * as THREE from "three";
import { THEME } from "../env/theme";

// Ball readability (cosmetic only; the sim radius is untouched): oversized
// neon ball, speed trail, floor shadow + marker ring and a vertical height
// cue for lobs. Allocation-free per frame.
const BALL_PHYS_R = 0.11; // mirrors sim ball radius
const BALL_VIS_SCALE = 1.4;
const BALL_R = BALL_PHYS_R * BALL_VIS_SCALE;
const BALL_COLOR = THEME.ball;
const BALL_EMISSIVE = THEME.ballEmissive;
const BALL_EMISSIVE_INTENSITY = 1.8; // pushes the ball over the bloom threshold
const SEGMENTS = 24;
const RINGS = 16;
const FLOOR_Y = 0.006;
const SHADOW_OPACITY = 0.5;
const SHADOW_MIN_SCALE = 0.45;
const SHADOW_FADE_HEIGHT = 6;
const MARKER_INNER = BALL_R * 1.6;
const MARKER_OUTER = BALL_R * 2.0;
const MARKER_OPACITY = 0.7;
const CUE_MIN_HEIGHT = 0.6; // below this the shadow alone is enough
const CUE_RADIUS = 0.012;
const CUE_OPACITY = 0.35;
const TRAIL_POINTS = 10;
const TRAIL_MIN_SPEED = 8; // m/s
const TRAIL_OPACITY = 0.55;
const OVERLAY_ORDER = 5;

function basic(color: number, opacity: number): THREE.MeshBasicMaterial {
  return new THREE.MeshBasicMaterial({ color, transparent: true, opacity, depthWrite: false });
}

export class BallView {
  private readonly ball: THREE.Mesh;
  private readonly shadow: THREE.Mesh;
  private readonly marker: THREE.Mesh;
  private readonly cue: THREE.Mesh;
  private readonly trail: THREE.Line;
  private readonly trailPos: Float32Array;
  private readonly trailAttr: THREE.BufferAttribute;
  private readonly last = new THREE.Vector3();
  private hasLast = false;

  constructor(scene: THREE.Scene) {
    this.ball = new THREE.Mesh(
      new THREE.SphereGeometry(BALL_R, SEGMENTS, RINGS),
      new THREE.MeshStandardMaterial({ color: BALL_COLOR, emissive: BALL_EMISSIVE, emissiveIntensity: BALL_EMISSIVE_INTENSITY, roughness: 0.4 }),
    );
    this.ball.castShadow = true;
    this.ball.renderOrder = OVERLAY_ORDER;
    this.shadow = this.floorMesh(new THREE.CircleGeometry(BALL_R * 1.2, SEGMENTS), basic(0x000000, SHADOW_OPACITY));
    this.marker = this.floorMesh(new THREE.RingGeometry(MARKER_INNER, MARKER_OUTER, SEGMENTS), basic(BALL_COLOR, MARKER_OPACITY));
    const cueGeo = new THREE.CylinderGeometry(CUE_RADIUS, CUE_RADIUS, 1, 6);
    cueGeo.translate(0, 0.5, 0); // unit height growing up from the floor
    this.cue = new THREE.Mesh(cueGeo, basic(BALL_COLOR, CUE_OPACITY));
    this.trailPos = new Float32Array(TRAIL_POINTS * 3);
    this.trailAttr = new THREE.BufferAttribute(this.trailPos, 3);
    this.trailAttr.setUsage(THREE.DynamicDrawUsage);
    const tg = new THREE.BufferGeometry();
    tg.setAttribute("position", this.trailAttr);
    this.trail = new THREE.Line(tg, new THREE.LineBasicMaterial({ color: BALL_COLOR, transparent: true, opacity: TRAIL_OPACITY }));
    this.trail.frustumCulled = false;
    scene.add(this.ball, this.shadow, this.marker, this.cue, this.trail);
  }

  get position(): THREE.Vector3 { return this.ball.position; }

  update(x: number, y: number, z: number, dt: number): void {
    this.ball.position.set(x, y, z);
    this.shadow.position.set(x, FLOOR_Y, z);
    const s = Math.max(SHADOW_MIN_SCALE, 1 - y / SHADOW_FADE_HEIGHT);
    this.shadow.scale.set(s, s, 1);
    this.marker.position.set(x, FLOOR_Y, z);
    this.cue.visible = y > CUE_MIN_HEIGHT;
    this.cue.position.set(x, 0, z);
    this.cue.scale.set(1, Math.max(y - BALL_R, 0.001), 1);
    this.updateTrail(x, y, z, dt);
  }

  private updateTrail(x: number, y: number, z: number, dt: number): void {
    const speed = this.hasLast && dt > 0 ? this.last.distanceTo(this.ball.position) / dt : 0;
    this.last.set(x, y, z);
    if (!this.hasLast) this.resetTrail(x, y, z);
    this.hasLast = true;
    const p = this.trailPos;
    p.copyWithin(3, 0, (TRAIL_POINTS - 1) * 3); // shift history back one point
    p[0] = x; p[1] = y; p[2] = z;
    this.trailAttr.needsUpdate = true;
    this.trail.visible = speed > TRAIL_MIN_SPEED;
  }

  private resetTrail(x: number, y: number, z: number): void {
    for (let i = 0; i < TRAIL_POINTS; i++) {
      this.trailPos[i * 3] = x; this.trailPos[i * 3 + 1] = y; this.trailPos[i * 3 + 2] = z;
    }
  }

  private floorMesh(geo: THREE.BufferGeometry, mat: THREE.Material): THREE.Mesh {
    const m = new THREE.Mesh(geo, mat);
    m.rotation.x = -Math.PI / 2;
    return m;
  }
}
