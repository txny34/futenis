import * as THREE from "three";

// Static court geometry. Units: meters. X long (-10..10), Z wide (-5..5), Y up.
const HALF_L = 10;
const HALF_W = 5;
const WALL_H = 3;
const NET_H = 0.88;
const SERVICE_X = 6.95;
const LINE_W = 0.05;
const LINE_Y = 0.002; // avoid z-fighting with the floor
const FLOOR_COLOR = 0x2563b8; // padel blue
const FLOOR_ROUGHNESS = 0.9;
const APRON_COLOR = 0x0a1a33;
const APRON_MARGIN = 6;
const APRON_Y = -0.01;
const LINE_COLOR = 0xffffff;
const GLASS_COLOR = 0xaad8ff;
const GLASS_FILL_OPACITY = 0.04;
const GLASS_EDGE_OPACITY = 0.45;
const RIVAL_GLASS_FILL_OPACITY = 0.09;
const RIVAL_EDGE_OPACITY = 0.7;
// Back wall behind team 0 sits between the camera and the court: a faint hint only.
const NEAR_GLASS_OPACITY = 0.015;
const NEAR_EDGE_OPACITY = 0.1;
const GLASS_EDGE_COLOR = 0xcfe8ff;
const NET_MESH_COLOR = 0x0b0f14;
const NET_MESH_OPACITY = 0.35;
const NET_BAND_H = 0.06;
const NET_BAND_T = 0.03;
const NET_BAND_COLOR = 0xffffff;
const POST_R = 0.04;
const POST_H = NET_H + 0.04;
const POST_COLOR = 0x2b2f36;

export function buildCourt(): THREE.Group {
  const g = new THREE.Group();
  g.add(apron(), floor(), lines(), walls(), net());
  return g;
}

function flat(w: number, d: number, color: number, y: number): THREE.Mesh {
  const m = new THREE.Mesh(new THREE.PlaneGeometry(w, d), new THREE.MeshStandardMaterial({ color, roughness: FLOOR_ROUGHNESS }));
  m.rotation.x = -Math.PI / 2;
  m.position.y = y;
  m.receiveShadow = true;
  return m;
}

const apron = (): THREE.Mesh => flat(2 * (HALF_L + APRON_MARGIN), 2 * (HALF_W + APRON_MARGIN), APRON_COLOR, APRON_Y);
const floor = (): THREE.Mesh => flat(2 * HALF_L, 2 * HALF_W, FLOOR_COLOR, 0);

/** Padel markings: service lines at ±6.95 (full width) and the center service line net->service line on each side. */
function lines(): THREE.Group {
  const g = new THREE.Group();
  const mat = new THREE.MeshBasicMaterial({ color: LINE_COLOR });
  const strip = (x: number, z: number, lx: number, lz: number): void => {
    const m = new THREE.Mesh(new THREE.PlaneGeometry(lx, lz), mat);
    m.rotation.x = -Math.PI / 2;
    m.position.set(x, LINE_Y, z);
    g.add(m);
  };
  strip(-SERVICE_X, 0, LINE_W, 2 * HALF_W);
  strip(SERVICE_X, 0, LINE_W, 2 * HALF_W);
  strip(-SERVICE_X / 2, 0, SERVICE_X, LINE_W);
  strip(SERVICE_X / 2, 0, SERVICE_X, LINE_W);
  return g;
}

function glassMat(opacity: number): THREE.MeshBasicMaterial {
  return new THREE.MeshBasicMaterial({ color: GLASS_COLOR, transparent: true, opacity, side: THREE.DoubleSide, depthWrite: false });
}

function walls(): THREE.Group {
  const g = new THREE.Group();
  const wall = (w: number, x: number, z: number, rotY: number, fill: number, edgeOpacity: number): void => {
    const m = new THREE.Mesh(new THREE.PlaneGeometry(w, WALL_H), glassMat(fill));
    m.position.set(x, WALL_H / 2, z);
    m.rotation.y = rotY;
    const edge = new THREE.LineSegments(
      new THREE.EdgesGeometry(m.geometry),
      new THREE.LineBasicMaterial({ color: GLASS_EDGE_COLOR, transparent: true, opacity: edgeOpacity }),
    );
    edge.position.copy(m.position);
    edge.rotation.copy(m.rotation);
    g.add(m, edge);
  };
  wall(2 * HALF_W, -HALF_L, 0, Math.PI / 2, NEAR_GLASS_OPACITY, NEAR_EDGE_OPACITY);
  wall(2 * HALF_W, HALF_L, 0, Math.PI / 2, RIVAL_GLASS_FILL_OPACITY, RIVAL_EDGE_OPACITY);
  wall(2 * HALF_L, 0, -HALF_W, 0, GLASS_FILL_OPACITY, GLASS_EDGE_OPACITY);
  wall(2 * HALF_L, 0, HALF_W, 0, GLASS_FILL_OPACITY, GLASS_EDGE_OPACITY);
  return g;
}

/** Thin translucent mesh + white top band + two posts. */
function net(): THREE.Group {
  const g = new THREE.Group();
  const mesh = new THREE.Mesh(
    new THREE.PlaneGeometry(2 * HALF_W, NET_H - NET_BAND_H),
    new THREE.MeshBasicMaterial({ color: NET_MESH_COLOR, transparent: true, opacity: NET_MESH_OPACITY, side: THREE.DoubleSide, depthWrite: false }),
  );
  mesh.rotation.y = Math.PI / 2;
  mesh.position.y = (NET_H - NET_BAND_H) / 2;
  const band = new THREE.Mesh(new THREE.BoxGeometry(NET_BAND_T, NET_BAND_H, 2 * HALF_W), new THREE.MeshBasicMaterial({ color: NET_BAND_COLOR }));
  band.position.y = NET_H - NET_BAND_H / 2;
  band.castShadow = true;
  g.add(mesh, band);
  const postMat = new THREE.MeshStandardMaterial({ color: POST_COLOR, roughness: 0.5 });
  for (const z of [-HALF_W, HALF_W]) {
    const post = new THREE.Mesh(new THREE.CylinderGeometry(POST_R, POST_R, POST_H, 12), postMat);
    post.position.set(0, POST_H / 2, z);
    post.castShadow = true;
    g.add(post);
  }
  return g;
}
