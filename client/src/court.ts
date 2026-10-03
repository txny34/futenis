import * as THREE from "three";
import { buildEnclosure } from "./env/enclosure";
import { THEME } from "./env/theme";

// Static court geometry. Units: meters. X long (-10..10), Z wide (-5..5), Y up.
const HALF_L = 10;
const HALF_W = 5;
const NET_H = 0.88;
const SERVICE_X = 6.95;
const LINE_W = 0.05;
const LINE_Y = 0.002; // avoid z-fighting with the floor
const FLOOR_ROUGHNESS = 0.9;
const APRON_MARGIN = 14; // ground runs under the stands and towers
const APRON_Y = -0.01;
// Lines are slightly over-bright so the bloom gives them a soft glow.
const LINE_BOOST = 1.25;
const NET_MESH_OPACITY = 0.6;
const NET_BAND_H = 0.06;
const NET_BAND_T = 0.03;
const POST_R = 0.04;
const POST_H = NET_H + 0.04;

export function buildCourt(): THREE.Group {
  const g = new THREE.Group();
  g.add(apron(), floor(), lines(), buildEnclosure(), net());
  return g;
}

function flat(w: number, d: number, color: number, y: number): THREE.Mesh {
  const m = new THREE.Mesh(new THREE.PlaneGeometry(w, d), new THREE.MeshStandardMaterial({ color, roughness: FLOOR_ROUGHNESS }));
  m.rotation.x = -Math.PI / 2;
  m.position.y = y;
  m.receiveShadow = true;
  return m;
}

const apron = (): THREE.Mesh => flat(2 * (HALF_L + APRON_MARGIN), 2 * (HALF_W + APRON_MARGIN), THEME.apron, APRON_Y);
const floor = (): THREE.Mesh => flat(2 * HALF_L, 2 * HALF_W, THEME.courtFloor, 0);

/** Padel markings: service lines at ±6.95 (full width) and the center service line net->service line on each side. */
function lines(): THREE.Group {
  const g = new THREE.Group();
  const mat = new THREE.MeshBasicMaterial({ color: new THREE.Color(THEME.courtLines).multiplyScalar(LINE_BOOST) });
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

/** Thin translucent mesh + white top band + two posts. */
function net(): THREE.Group {
  const g = new THREE.Group();
  const mesh = new THREE.Mesh(
    new THREE.PlaneGeometry(2 * HALF_W, NET_H - NET_BAND_H),
    new THREE.MeshBasicMaterial({ color: THEME.netMesh, transparent: true, opacity: NET_MESH_OPACITY, side: THREE.DoubleSide, depthWrite: false }),
  );
  mesh.rotation.y = Math.PI / 2;
  mesh.position.y = (NET_H - NET_BAND_H) / 2;
  const band = new THREE.Mesh(new THREE.BoxGeometry(NET_BAND_T, NET_BAND_H, 2 * HALF_W), new THREE.MeshBasicMaterial({ color: THEME.netBand }));
  band.position.y = NET_H - NET_BAND_H / 2;
  band.castShadow = true;
  g.add(mesh, band);
  const postMat = new THREE.MeshStandardMaterial({ color: THEME.metal, roughness: 0.5 });
  for (const z of [-HALF_W, HALF_W]) {
    const post = new THREE.Mesh(new THREE.CylinderGeometry(POST_R, POST_R, POST_H, 12), postMat);
    post.position.set(0, POST_H / 2, z);
    post.castShadow = true;
    g.add(post);
  }
  return g;
}
