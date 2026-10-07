import * as THREE from "three";
import { THEME } from "./theme";

// Visual padel enclosure (sim walls are unchanged): framed 2 m glass panels,
// 3 m tall on both back walls and the first 2 m of each side wall, then the
// 2 m tall "escalón" panel, metal mesh above the glass up to 4 m / 3 m, mesh
// on the middle of the sides, dark metal posts. The near back wall (behind the
// human camera) gets no mesh and almost invisible glass so it never blocks view.
const HALF_L = 10;
const HALF_W = 5;
const GLASS_H = 3;
const FENCE_TOP = 4;
const SIDE_GLASS_LEN = 2;
const STEP_GLASS_H = 2; // second side panel, one step lower
const PANEL_W = 2; // glass is split into framed panels this wide
const MID_MESH_H = 3;
const POST_W = 0.08;
const POST_SPACING = 2;
const TEX_PX = 64;
const WIRE_PX = 9;
const MESH_CELL_M = 0.08; // one texture tile = 8 cm grid
const MESH_ALPHA_TEST = 0.4;
const GLASS_OPACITY = 0.17;
const NEAR_GLASS_OPACITY = 0.03;
const EDGE_OPACITY = 0.9;
const NEAR_EDGE_OPACITY = 0.08;
const METAL_ROUGHNESS = 0.45;
const METAL_METALNESS = 0.7;
const SPARSE_MESH_OPACITY = 0.22;

/** Match court: defaults. Lobby showcase: both back walls solid, see-through mesh on one side. */
export interface EnclosureOpts {
  /** Both back walls get full glass + top mesh (no faded near wall behind the camera). */
  readonly symmetric?: boolean;
  /** Side (sign of Z) whose wire mesh is faint so the court reads through it. */
  readonly sparseSideZ?: -1 | 1;
}

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

function glassPanel(m: Mats, w: number, h: number, x: number, z: number, alongZ: boolean, near: boolean): THREE.Object3D[] {
  const out: THREE.Object3D[] = [];
  const n = Math.max(1, Math.round(w / PANEL_W));
  const pw = w / n;
  const geo = new THREE.PlaneGeometry(pw, h);
  const edges = new THREE.EdgesGeometry(geo);
  for (let i = 0; i < n; i++) {
    const off = -w / 2 + pw * (i + 0.5);
    const px = alongZ ? x : x + off;
    const pz = alongZ ? z + off : z;
    const pane = new THREE.Mesh(geo, near ? m.nearGlass : m.glass);
    const edge = new THREE.LineSegments(edges, near ? m.nearEdge : m.edge);
    place(pane, h, px, 0, pz, alongZ);
    place(edge, h, px, 0, pz, alongZ);
    out.push(pane, edge);
  }
  return out;
}

function meshPanel(m: Mats, w: number, h: number, x: number, y0: number, z: number, alongZ: boolean, sparse = false): THREE.Mesh {
  const tex = m.wire.clone();
  tex.repeat.set(w / MESH_CELL_M, h / MESH_CELL_M);
  tex.needsUpdate = true;
  const mat = new THREE.MeshBasicMaterial({ color: THEME.mesh, alphaMap: tex, alphaTest: MESH_ALPHA_TEST, side: THREE.DoubleSide });
  if (sparse) { mat.transparent = true; mat.opacity = SPARSE_MESH_OPACITY; mat.depthWrite = false; }
  const panel = new THREE.Mesh(new THREE.PlaneGeometry(w, h), mat);
  place(panel, h, x, y0, z, alongZ);
  return panel;
}

function posts(m: Mats, symmetric: boolean): THREE.InstancedMesh {
  const spots: Array<readonly [number, number, number]> = [];
  for (let x = -HALF_L; x <= HALF_L + 1e-6; x += POST_SPACING) {
    const h = Math.abs(x) >= HALF_L - SIDE_GLASS_LEN ? FENCE_TOP : MID_MESH_H;
    spots.push([x, -HALF_W, h], [x, HALF_W, h]);
  }
  for (let z = -HALF_W + POST_SPACING; z < HALF_W; z += POST_SPACING) {
    spots.push([HALF_L, z, FENCE_TOP]);
    if (symmetric) spots.push([-HALF_L, z, FENCE_TOP]);
  }
  const inst = new THREE.InstancedMesh(new THREE.BoxGeometry(POST_W, 1, POST_W), m.metal, spots.length);
  const mtx = new THREE.Matrix4();
  spots.forEach(([x, z, h], i) => {
    mtx.makeScale(1, h, 1).setPosition(x, h / 2, z);
    inst.setMatrixAt(i, mtx);
  });
  inst.castShadow = true;
  return inst;
}

export function buildEnclosure(opts: EnclosureOpts = {}): THREE.Group {
  const m = makeMats();
  const g = new THREE.Group();
  const backW = 2 * HALF_W;
  const midHalf = HALF_L - 2 * SIDE_GLASS_LEN;
  g.add(...glassPanel(m, backW, GLASS_H, -HALF_L, 0, true, !opts.symmetric));
  if (opts.symmetric) g.add(meshPanel(m, backW, FENCE_TOP - GLASS_H, -HALF_L, GLASS_H, 0, true));
  g.add(...glassPanel(m, backW, GLASS_H, HALF_L, 0, true, false));
  g.add(meshPanel(m, backW, FENCE_TOP - GLASS_H, HALF_L, GLASS_H, 0, true));
  for (const z of [-HALF_W, HALF_W]) {
    for (const sx of [-1, 1]) {
      const near = sx < 0 && !opts.symmetric; // near-side glass fades so it never frames the view
      const sparse = Math.sign(z) === opts.sparseSideZ;
      const cx = sx * (HALF_L - SIDE_GLASS_LEN / 2);
      const stepX = sx * (HALF_L - SIDE_GLASS_LEN * 1.5);
      g.add(...glassPanel(m, SIDE_GLASS_LEN, GLASS_H, cx, z, false, near));
      g.add(meshPanel(m, SIDE_GLASS_LEN, FENCE_TOP - GLASS_H, cx, GLASS_H, z, false, sparse));
      g.add(...glassPanel(m, SIDE_GLASS_LEN, STEP_GLASS_H, stepX, z, false, near));
      g.add(meshPanel(m, SIDE_GLASS_LEN, MID_MESH_H - STEP_GLASS_H, stepX, STEP_GLASS_H, z, false, sparse));
    }
    g.add(meshPanel(m, 2 * midHalf, MID_MESH_H, 0, 0, z, false, Math.sign(z) === opts.sparseSideZ));
  }
  g.add(posts(m, opts.symmetric === true));
  return g;
}
