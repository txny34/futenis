import * as THREE from "three";
import { CharacterPose, DEFAULT_LOOK, createCharacter } from "../../character/character";
import { loadStyle, onStyleChange } from "../../profile/style-store";
import { THEME } from "../../env/theme";
import { FONT_STACK, canvasTexture } from "../decor/util";
import { t } from "../../i18n/t";

// Podium blocks 1-2-3 under a side board. Only #1 can hold a statue: the
// player's own character rig, frozen in an idle pose, shown once they have
// at least one win; otherwise an empty block with "¿Serás vos?". Static:
// no per-frame work (the rig is posed once at build time).
const BLOCK_W = 1.3;
const BLOCK_D = 1.2;
const HEIGHTS: readonly [number, number, number] = [0.9, 0.6, 0.4];
const XS: readonly [number, number, number] = [0, -1.4, 1.4]; // #1 center, #2 left, #3 right
const TRIM_H = 0.06;
const MEDAL_DIM = 0.55;
const STATUE_SCALE = 1.1;
const POSE_STEPS = 30;
const POSE_DT = 1 / 60;
const INVITE_W = 2.2;
const INVITE_Y = 1.7;
const H_ = THEME.lobbyDecor.hall;

function numberTex(n: number): THREE.CanvasTexture {
  return canvasTexture(128, 128, (c) => {
    c.clearRect(0, 0, 128, 128);
    c.fillStyle = H_.ownText;
    c.font = `96px ${FONT_STACK}`;
    c.textAlign = "center";
    c.textBaseline = "middle";
    c.fillText(String(n), 64, 70);
  });
}

function inviteTex(): THREE.CanvasTexture {
  return canvasTexture(512, 128, (c) => {
    c.clearRect(0, 0, 512, 128);
    c.fillStyle = H_.signText;
    c.font = `64px ${FONT_STACK}`;
    c.textAlign = "center";
    c.textBaseline = "middle";
    c.fillText(t("¿Serás vos?", "Could it be you?"), 256, 66, 496);
  });
}

export class Podium {
  readonly group = new THREE.Group();
  private readonly statue: THREE.Object3D;
  private readonly invite: THREE.Mesh;

  constructor() {
    const body = new THREE.MeshStandardMaterial({ color: H_.podium, roughness: 0.45, metalness: 0.5 });
    for (let i = 0; i < 3; i++) this.buildBlock(i, body);
    const ch = createCharacter(DEFAULT_LOOK); // the player's own look (view/players.ts)
    ch.setStyle(loadStyle());
    onStyleChange((st) => ch.setStyle(st)); // Estilos edits show on the statue too
    const pose: CharacterPose = { speed: 0, airborne: false, verticalVel: 0, charge: 0, strike: null, facing: Math.PI };
    for (let i = 0; i < POSE_STEPS; i++) ch.update(pose, POSE_DT); // settle into idle, then freeze
    this.statue = ch.root;
    this.statue.scale.setScalar(STATUE_SCALE);
    this.statue.position.set(XS[0], HEIGHTS[0], 0);
    this.invite = new THREE.Mesh(new THREE.PlaneGeometry(INVITE_W, INVITE_W / 4),
      new THREE.MeshBasicMaterial({ map: inviteTex(), transparent: true, depthWrite: false }));
    this.invite.position.set(XS[0], INVITE_Y, 0);
    this.invite.rotation.y = Math.PI; // faces -Z (toward the lobby)
    this.group.add(this.statue, this.invite);
    this.setChampion(false);
  }

  /** Statue on #1 when the player has won at least once. Not per frame. */
  setChampion(on: boolean): void {
    this.statue.visible = on;
    this.invite.visible = !on;
  }

  private buildBlock(i: number, body: THREE.Material): void {
    const h = HEIGHTS[i];
    const block = new THREE.Mesh(new THREE.BoxGeometry(BLOCK_W, h, BLOCK_D), body);
    block.position.set(XS[i], h / 2, 0);
    const trim = new THREE.Mesh(new THREE.BoxGeometry(BLOCK_W + 0.02, TRIM_H, BLOCK_D + 0.02),
      new THREE.MeshBasicMaterial({ color: new THREE.Color(H_.medals[i]).multiplyScalar(MEDAL_DIM) }));
    trim.position.set(XS[i], h - TRIM_H, 0);
    const num = new THREE.Mesh(new THREE.PlaneGeometry(0.5, 0.5),
      new THREE.MeshBasicMaterial({ map: numberTex(i + 1), transparent: true, depthWrite: false }));
    num.position.set(XS[i], h / 2, -BLOCK_D / 2 - 0.01);
    num.rotation.y = Math.PI;
    this.group.add(block, trim, num);
  }
}
