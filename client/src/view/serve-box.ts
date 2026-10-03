import * as THREE from "three";

// Highlights the diagonal service box the human must serve into.
// Box: rival half between the net (X=0) and the service line (X=6.95),
// on the Z half opposite to the server (crosswise).
const SERVICE_X = 6.95;
const HALF_W = 5;
const FLOOR_Y = 0.004;
const COLOR = 0xffd23f;
const OPACITY = 0.22;
const PULSE_HZ = 1.5;
const PULSE_AMOUNT = 0.1;

export class ServeBox {
  private readonly mesh: THREE.Mesh;
  private readonly mat = new THREE.MeshBasicMaterial({ color: COLOR, transparent: true, opacity: OPACITY, depthWrite: false });
  private time = 0;

  constructor(scene: THREE.Scene) {
    this.mesh = new THREE.Mesh(new THREE.PlaneGeometry(SERVICE_X, HALF_W), this.mat);
    this.mesh.rotation.x = -Math.PI / 2;
    this.mesh.visible = false;
    scene.add(this.mesh);
  }

  /** serverZ = serving player's Z; the box is on the opposite Z half of the rival side. */
  update(visible: boolean, serverZ: number, dt: number): void {
    this.mesh.visible = visible;
    if (!visible) return;
    this.time += dt;
    const zSign = serverZ >= 0 ? -1 : 1;
    this.mesh.position.set(SERVICE_X / 2, FLOOR_Y, (zSign * HALF_W) / 2);
    this.mat.opacity = OPACITY + Math.sin(this.time * Math.PI * 2 * PULSE_HZ) * PULSE_AMOUNT;
  }
}
