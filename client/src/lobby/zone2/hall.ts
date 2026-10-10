import * as THREE from "three";
import { THEME } from "../../env/theme";
import { ProfileStats, profileStore } from "../../profile/store";
import { FONT_STACK, canvasTexture } from "../decor/util";
import { Board } from "./board";
import { Podium } from "./podium";
import { t } from "../../i18n/t";

// Zona 2 · Salón de la fama (docs/design/lobby.md §7.2), at the +Z end of the
// lobby, behind spawn. Three framed boards (ELO 1v1, top wins, ELO 2v2) facing
// -Z, podiums 1-2-3 under the side boards, a floor arrow near spawn. Local
// data only; everything redraws on profile onChange, nothing per frame.
const BOARD_Z = 22.4; // just in front of the back wall (decor/arches.ts WALL_BACK_Z = 23)
const CENTER_W = 4.2;
const SIDE_W = 3.6;
const CENTER_Y = 4.0;
const SIDE_Y = 4.6;
const SIDE_X = 8;
const SIDE_TURN = 0.35; // side boards angle in toward the center
const PODIUM_Z = 19.6; // past the walkable edge (LobbyHalfZ = 18): no walking through
const TITLE = { y: 7.6, w: 8 } as const;
const SIGN = { z: 10.5, w: 4.2, d: 2.1 } as const;
const LIGHT = { y: 9, z: 14, intensity: 6, dist: 22, angle: Math.PI / 4 } as const;
const H_ = THEME.lobbyDecor.hall;

function titleTex(): THREE.CanvasTexture {
  return canvasTexture(1024, 128, (c) => {
    c.clearRect(0, 0, 1024, 128);
    c.fillStyle = H_.signText;
    c.font = `84px ${FONT_STACK}`;
    c.textAlign = "center";
    c.textBaseline = "middle";
    c.fillText(t("SALÓN DE LA FAMA", "HALL OF FAME"), 512, 68, 1000);
  });
}

/** Floor arrow (canvas up = +Z once laid down): read when turning around from spawn. */
function signTex(): THREE.CanvasTexture {
  return canvasTexture(512, 256, (c) => {
    c.clearRect(0, 0, 512, 256);
    c.fillStyle = H_.signText;
    c.globalAlpha = 0.75;
    c.beginPath();
    c.moveTo(256, 8);
    c.lineTo(316, 78);
    c.lineTo(196, 78);
    c.closePath();
    c.fill();
    c.font = `44px ${FONT_STACK}`;
    c.textAlign = "center";
    c.textBaseline = "middle";
    c.fillText(t("ZONA 2", "ZONE 2"), 256, 128, 490);
    c.font = `38px ${FONT_STACK}`;
    c.fillText(t("SALÓN DE LA FAMA", "HALL OF FAME"), 256, 196, 490);
  });
}

export class HallOfFame {
  readonly group = new THREE.Group();
  /** Constant light set (LobbyDecor.setLit dims it outside the lobby). */
  readonly lights = new THREE.Group();
  private readonly elo1 = new Board(t("MEJOR ELO 1v1", "BEST ELO 1v1"), SIDE_W);
  private readonly wins = new Board(t("TOP VICTORIAS", "TOP WINS"), CENTER_W);
  private readonly elo2 = new Board(t("MEJOR ELO 2v2", "BEST ELO 2v2"), SIDE_W);
  private readonly podiums: readonly [Podium, Podium] = [new Podium(), new Podium()];

  constructor() {
    this.place(this.wins.group, 0, CENTER_Y, BOARD_Z, Math.PI);
    this.place(this.elo1.group, -SIDE_X, SIDE_Y, BOARD_Z - 0.6, Math.PI - SIDE_TURN);
    this.place(this.elo2.group, SIDE_X, SIDE_Y, BOARD_Z - 0.6, Math.PI + SIDE_TURN);
    this.place(this.podiums[0].group, -SIDE_X, 0, PODIUM_Z, -SIDE_TURN);
    this.place(this.podiums[1].group, SIDE_X, 0, PODIUM_Z, SIDE_TURN);
    const title = new THREE.Mesh(new THREE.PlaneGeometry(TITLE.w, TITLE.w / 8),
      new THREE.MeshBasicMaterial({ map: titleTex(), transparent: true, depthWrite: false }));
    this.place(title, 0, TITLE.y, BOARD_Z, Math.PI);
    const sign = new THREE.Mesh(new THREE.PlaneGeometry(SIGN.w, SIGN.d),
      new THREE.MeshBasicMaterial({ map: signTex(), transparent: true, depthWrite: false }));
    sign.rotation.set(-Math.PI / 2, 0, Math.PI); // flat, text up toward +Z
    sign.position.set(0, 0.02, SIGN.z);
    this.group.add(sign);
    const l = new THREE.SpotLight(H_.light, LIGHT.intensity, LIGHT.dist, LIGHT.angle, 0.6, 1);
    l.position.set(0, LIGHT.y, LIGHT.z);
    l.target.position.set(0, 2, BOARD_Z - 2);
    this.lights.add(l, l.target);
    const store = profileStore();
    this.apply(store.get());
    store.onChange((s) => this.apply(s)); // lives as long as the page (lobby is built once)
  }

  private place(o: THREE.Object3D, x: number, y: number, z: number, ry: number): void {
    o.position.set(x, y, z);
    o.rotation.y = ry;
    this.group.add(o);
  }

  private apply(s: Readonly<ProfileStats>): void {
    this.elo1.setValue(s.name, s.elo["1v1"]);
    this.wins.setValue(s.name, s.wins);
    this.elo2.setValue(s.name, s.elo["2v2"]);
    const champ = s.wins >= 1;
    for (const p of this.podiums) p.setChampion(champ);
  }
}
