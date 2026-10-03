import * as THREE from "three";

// Static court geometry. Units: meters. X long (-10..10), Z wide (-5..5), Y up.
const HALF_L = 10;
const HALF_W = 5;
const WALL_H = 3;
const NET_H = 0.88;
const SERVICE_X = 6.95;
const LINE_W = 0.05;
const LINE_Y = 0.002; // avoid z-fighting with the floor
const FLOOR_COLOR = 0x1f6f8b;
const FLOOR_ROUGHNESS = 0.9;
const LINE_COLOR = 0xffffff;
const GLASS_COLOR = 0xaad8ff;
const GLASS_OPACITY = 0.12;
const GLASS_ROUGHNESS = 0.1;
const GLASS_EDGE_COLOR = 0xcfe8ff;
const GLASS_EDGE_OPACITY = 0.5;
// Back wall behind player 0 sits between the camera and the court: keep it a faint hint.
const NEAR_GLASS_OPACITY = 0.02;
const NEAR_EDGE_OPACITY = 0.08;
const NET_THICKNESS = 0.02;
const NET_COLOR = 0xeeeeee;
const NET_OPACITY = 0.7;

export function buildCourt(): THREE.Group {
  const g = new THREE.Group();
  g.add(floor(), lines(), walls(), net());
  return g;
}

function floor(): THREE.Mesh {
  const m = new THREE.Mesh(
    new THREE.PlaneGeometry(2 * HALF_L, 2 * HALF_W),
    new THREE.MeshStandardMaterial({ color: FLOOR_COLOR, roughness: FLOOR_ROUGHNESS }),
  );
  m.rotation.x = -Math.PI / 2;
  m.receiveShadow = true;
  return m;
}

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
  strip(0, 0, 2 * SERVICE_X, LINE_W);
  return g;
}

function walls(): THREE.Group {
  const g = new THREE.Group();
  const glass = (opacity: number): THREE.MeshPhysicalMaterial =>
    new THREE.MeshPhysicalMaterial({
      color: GLASS_COLOR,
      transparent: true,
      opacity,
      roughness: GLASS_ROUGHNESS,
      side: THREE.DoubleSide,
      depthWrite: false,
    });
  const mat = glass(GLASS_OPACITY);
  const nearMat = glass(NEAR_GLASS_OPACITY);
  const wall = (w: number, x: number, z: number, rotY: number, near = false): void => {
    const m = new THREE.Mesh(new THREE.PlaneGeometry(w, WALL_H), near ? nearMat : mat);
    m.position.set(x, WALL_H / 2, z);
    m.rotation.y = rotY;
    g.add(m);
    const edge = new THREE.LineSegments(
      new THREE.EdgesGeometry(m.geometry),
      new THREE.LineBasicMaterial({ color: GLASS_EDGE_COLOR, transparent: true, opacity: near ? NEAR_EDGE_OPACITY : GLASS_EDGE_OPACITY }),
    );
    edge.position.copy(m.position);
    edge.rotation.copy(m.rotation);
    g.add(edge);
  };
  wall(2 * HALF_W, -HALF_L, 0, Math.PI / 2, true);
  wall(2 * HALF_W, HALF_L, 0, Math.PI / 2);
  wall(2 * HALF_L, 0, -HALF_W, 0);
  wall(2 * HALF_L, 0, HALF_W, 0);
  return g;
}

function net(): THREE.Mesh {
  const m = new THREE.Mesh(
    new THREE.BoxGeometry(NET_THICKNESS, NET_H, 2 * HALF_W),
    new THREE.MeshStandardMaterial({ color: NET_COLOR, transparent: true, opacity: NET_OPACITY }),
  );
  m.position.set(0, NET_H / 2, 0);
  m.castShadow = true;
  return m;
}
