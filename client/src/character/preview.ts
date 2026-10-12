import * as THREE from "three";
import { DEFAULT_STYLE, createCharacter, styleFromSeed } from "./character";
import { BODY, DEFAULT_LOOK } from "./config";
import { createHitboxOverlay } from "./debug";
import { createDriver } from "./preview-driver";
import { buildControls } from "./preview-ui";

// Standalone character preview: game-like high camera (left) + close-up
// (right) rendered with scissor viewports into one canvas.

const BG = 0x0d1117;
const FLOOR_COLOR = 0x2f6f4f;
const FLOOR_SIZE = 6;
const BALL_R = 0.11;
const BALL_COLOR = 0xf5e663;
const GAME_OFFSET = new THREE.Vector3(-11, 11, 0); // render.ts: cam (-19,11,0), player ~X=-8
const CLOSEUP_DIST = 3.2;
const CLOSEUP_HEIGHT = 1.3;
const CLOSEUP_TARGET = new THREE.Vector3(0, 1.0, 0);
const MAX_DT = 0.1;
const MAX_PIXEL_RATIO = 2;
const SPLIT = 0.5;

function buildScene(): { scene: THREE.Scene; ball: THREE.Mesh } {
  const scene = new THREE.Scene();
  scene.background = new THREE.Color(BG);
  scene.add(new THREE.HemisphereLight(0xdbe9ff, 0x223344, 0.9));
  const sun = new THREE.DirectionalLight(0xffffff, 1.6);
  sun.position.set(-4, 8, 5);
  sun.castShadow = true;
  scene.add(sun);
  const floor = new THREE.Mesh(
    new THREE.PlaneGeometry(FLOOR_SIZE, FLOOR_SIZE),
    new THREE.MeshToonMaterial({ color: FLOOR_COLOR }),
  );
  floor.rotation.x = -Math.PI / 2;
  floor.receiveShadow = true;
  scene.add(floor, new THREE.GridHelper(FLOOR_SIZE, FLOOR_SIZE * 2, 0x446655, 0x335544));
  const ball = new THREE.Mesh(
    new THREE.SphereGeometry(BALL_R, 16, 12),
    new THREE.MeshToonMaterial({ color: BALL_COLOR }),
  );
  scene.add(ball);
  return { scene, ball };
}

function main(): void {
  const app = document.getElementById("app");
  const ui = document.getElementById("ui");
  if (!app || !ui) throw new Error("character.html: #app/#ui missing");
  const host: HTMLElement = app;
  const renderer = new THREE.WebGLRenderer({ antialias: true });
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, MAX_PIXEL_RATIO));
  renderer.shadowMap.enabled = true;
  renderer.setScissorTest(true);
  host.appendChild(renderer.domElement);

  const { scene, ball } = buildScene();
  const character = createCharacter(DEFAULT_LOOK);
  scene.add(character.root);
  const overlay = createHitboxOverlay();
  scene.add(overlay);

  const gameCam = new THREE.PerspectiveCamera(50, 1, 0.1, 100);
  gameCam.position.copy(GAME_OFFSET);
  gameCam.lookAt(0, BODY.height / 2, 0);
  const closeCam = new THREE.PerspectiveCamera(35, 1, 0.1, 50);
  const FACE_CLOSEUP = 4; // last close-up angle: the face, for the expressions
  const FACE_DIST = 0.85;

  const info = `${Math.round(character.triangles)} tris · ${Object.keys(character.rig.bones).length} bones`;
  const ctl = buildControls(ui, info);
  const driver = createDriver();

  const headPos = new THREE.Vector3();
  const headDir = new THREE.Vector3();
  function placeCloseup(): void {
    if (ctl.closeupAngle === FACE_CLOSEUP) {
      const head = character.rig.bones.head;
      head.getWorldPosition(headPos);
      head.getWorldDirection(headDir); // the face looks down the head's +Z
      headPos.y += BODY.headCenter - BODY.headBase;
      closeCam.position.copy(headPos).addScaledVector(headDir, FACE_DIST);
      closeCam.position.y += 0.03;
      closeCam.lookAt(headPos);
      return;
    }
    const a = ctl.closeupAngle * (Math.PI / 2) + Math.PI / 4;
    closeCam.position.set(Math.cos(a) * CLOSEUP_DIST, CLOSEUP_HEIGHT, Math.sin(a) * CLOSEUP_DIST);
    closeCam.lookAt(CLOSEUP_TARGET);
  }

  function draw(cam: THREE.PerspectiveCamera, x: number, w: number, h: number): void {
    cam.aspect = w / h;
    cam.updateProjectionMatrix();
    renderer.setViewport(x, 0, w, h);
    renderer.setScissor(x, 0, w, h);
    renderer.render(scene, cam);
  }

  const size = new THREE.Vector2();
  let last = performance.now();
  let clock = 0;
  let styleSeed = 0;
  function frame(now: number): void {
    const dt = ctl.paused ? 0 : Math.min(MAX_DT, (now - last) / 1000);
    last = now;
    clock += dt;
    driver.step(ctl.state, ctl.speed, clock, ctl.scrub, ctl.side);
    const f = driver.frame;
    character.root.position.y = f.rootY;
    if (ctl.styleSeed !== styleSeed) {
      styleSeed = ctl.styleSeed;
      character.setStyle(styleSeed === 0 ? DEFAULT_STYLE : styleFromSeed(styleSeed));
    }
    if (ctl.emotion !== null) character.express(ctl.emotion, 0.2); // held while picked
    character.update(f.pose, dt);
    ball.visible = f.ballVisible;
    ball.position.copy(f.ball);
    overlay.visible = ctl.showHitbox;
    overlay.position.y = f.rootY;
    placeCloseup();
    const w = host.clientWidth;
    const h = host.clientHeight;
    renderer.getSize(size);
    if (size.x !== w || size.y !== h) renderer.setSize(w, h, false);
    const left = Math.floor(w * SPLIT);
    draw(gameCam, 0, left, h);
    draw(closeCam, left, w - left, h);
    requestAnimationFrame(frame);
  }
  requestAnimationFrame(frame);
}

main();
