import * as THREE from "three";
import { THEME } from "./theme";

// Visual padel enclosure (sim walls are unchanged): 3 m glass on both back
// walls and the first 2 m of each side wall, metal mesh up to 4 m, mesh on
// the middle of the sides, dark metal posts. The near back wall (behind the
// human camera) gets no mesh and almost invisible glass so it never blocks view.
const HALF_L = 10;
const HALF_W = 5;
const GLASS_H = 3;
const FENCE_TOP = 4;
const SIDE_GLASS_LEN = 2;
const MID_MESH_H = 3;
const POST_W = 0.08;
const POST_SPACING = 2;
const TEX_PX = 64;
const WIRE_PX = 5;
const MESH_CELL_M = 0.1; // one texture tile = 10 cm grid
const MESH_ALPHA_TEST = 0.4;
const GLASS_OPACITY = 0.07;
const NEAR_GLASS_OPACITY = 0.015;
const EDGE_OPACITY = 0.55;
const NEAR_EDGE_OPACITY = 0.08;
const METAL_ROUGHNESS = 0.45;
const METAL_METALNESS = 0.7;

function wireTexture(): THREE.CanvasTexture {
  const c = document.createElement("canvas");
  c.width = TEX_PX;
  c.height = TEX_PX;
  const g = c.getContext("2d");
  if (!g) throw new Error("canvas 2d no disponible");
  g.fillStyle = "#fff";
  g.fillRect(0, 0, TEX_PX, WIRE_PX);
  g.fillRect(0, 0, WIRE_PX, TEX_PX);
  const tex = new THREE.CanvasTexture(c);
  tex.wrapS = THREE.RepeatWrapping;
  tex.wrapT = THREE.RepeatWrapping;
  tex.anisotropy = 4;
  return tex;
}

interface Mats {
  readonly glass: THREE.MeshBasicMaterial;
  readonly nearGlass: THREE.MeshBasicMaterial;
  readonly edge: THREE.LineBasicMaterial;
  readonly nearEdge: THREE.LineBasicMaterial;
  readonly metal: THREE.MeshStandardMaterial;
  readonly wire: THREE.CanvasTexture;
}

function makeMats(): Mats {
  const glass = (opacity: number): THREE.MeshBasicMaterial =>
    new THREE.MeshBasicMaterial({ color: THEME.glass, transparent: true, opacity, side: THREE.DoubleSide, depthWrite: false });
  const edge = (opacity: number): THREE.LineBasicMaterial =>
    new THREE.LineBasicMaterial({ color: THEME.glassEdge, transparent: true, opacity });
  return {
    glass: glass(GLASS_OPACITY),
    nearGlass: glass(NEAR_GLASS_OPACITY),
    edge: edge(EDGE_OPACITY),
    nearEdge: edge(NEAR_EDGE_OPACITY),
    metal: new THREE.MeshStandardMaterial({ color: THEME.metal, roughness: METAL_ROUGHNESS, metalness: METAL_METALNESS }),
    wire: wireTexture(),
  };
}

/** Panel centered at (x, y0 + h/2, z), spanning `w` along X (rotY 0) or Z (rotY PI/2). */
function place(o: THREE.Object3D, h: number, x: number, y0: number, z: number, alongZ: boolean): void {
  o.position.set(x, y0 + h / 2, z);
  o.rotation.y = alongZ ? Math.PI / 2 : 0;
}

function glassPanel(m: Mats, w: number, x: number, z: number, alongZ: boolean, near: boolean): THREE.Object3D[] {
  const geo = new THREE.PlaneGeometry(w, GLASS_H);
  const pane = new THREE.Mesh(geo, near ? m.nearGlass : m.glass);
  const edge = new THREE.LineSegments(new THREE.EdgesGeometry(geo), near ? m.nearEdge : m.edge);
  place(pane, GLASS_H, x, 0, z, alongZ);
  place(edge, GLASS_H, x, 0, z, alongZ);
  return [pane, edge];
}

function meshPanel(m: Mats, w: number, h: number, x: number, y0: number, z: number, alongZ: boolean): THREE.Mesh {
  const tex = m.wire.clone();
  tex.repeat.set(w / MESH_CELL_M, h / MESH_CELL_M);
  tex.needsUpdate = true;
  const mat = new THREE.MeshBasicMaterial({ color: THEME.mesh, alphaMap: tex, alphaTest: MESH_ALPHA_TEST, side: THREE.DoubleSide });
  const panel = new THREE.Mesh(new THREE.PlaneGeometry(w, h), mat);
  place(panel, h, x, y0, z, alongZ);
  return panel;
}

function posts(m: Mats): THREE.InstancedMesh {
  const spots: Array<readonly [number, number, number]> = [];
  for (let x = -HALF_L; x <= HALF_L + 1e-6; x += POST_SPACING) {
    const h = Math.abs(x) >= HALF_L - SIDE_GLASS_LEN ? FENCE_TOP : MID_MESH_H;
    spots.push([x, -HALF_W, h], [x, HALF_W, h]);
  }
  for (let z = -HALF_W + POST_SPACING; z < HALF_W; z += POST_SPACING) spots.push([HALF_L, z, FENCE_TOP]);
  const inst = new THREE.InstancedMesh(new THREE.BoxGeometry(POST_W, 1, POST_W), m.metal, spots.length);
  const mtx = new THREE.Matrix4();
  spots.forEach(([x, z, h], i) => {
    mtx.makeScale(1, h, 1).setPosition(x, h / 2, z);
    inst.setMatrixAt(i, mtx);
  });
  inst.castShadow = true;
  return inst;
}

export function buildEnclosure(): THREE.Group {
  const m = makeMats();
  const g = new THREE.Group();
  const backW = 2 * HALF_W;
  const midLen = 2 * (HALF_L - SIDE_GLASS_LEN);
  g.add(...glassPanel(m, backW, -HALF_L, 0, true, true));
  g.add(...glassPanel(m, backW, HALF_L, 0, true, false));
  g.add(meshPanel(m, backW, FENCE_TOP - GLASS_H, HALF_L, GLASS_H, 0, true));
  for (const z of [-HALF_W, HALF_W]) {
    for (const sx of [-1, 1]) {
      const cx = sx * (HALF_L - SIDE_GLASS_LEN / 2);
      g.add(...glassPanel(m, SIDE_GLASS_LEN, cx, z, false, sx < 0)); // near-side glass fades so it never frames the view
      g.add(meshPanel(m, SIDE_GLASS_LEN, FENCE_TOP - GLASS_H, cx, GLASS_H, z, false));
    }
    g.add(meshPanel(m, midLen, MID_MESH_H, 0, 0, z, false));
  }
  g.add(posts(m));
  return g;
}
