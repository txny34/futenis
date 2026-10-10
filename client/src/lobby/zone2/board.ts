import * as THREE from "three";
import { THEME } from "../../env/theme";
import { FONT_STACK } from "../decor/util";
import { t } from "../../i18n/t";

// One framed leaderboard screen of the hall (lobby/zone2). Local data only:
// the guest's own highlighted row plus clearly-marked "coming soon" slots
// where the online top list will go (no invented names). Redrawn only when
// the profile changes (setValue), never per frame.
const W = 768;
const H = 1024;
const SLOTS = 7;
const SLOT_Y0 = 250;
const SLOT_H = 74;
const OWN_Y = 840;
const OWN_H = 110;
const FRAME = 0.18;
const FACE_DIM = 0.8; // calm: under the bloom threshold
const H_ = THEME.lobbyDecor.hall;

export class Board {
  readonly group = new THREE.Group();
  private readonly ctx: CanvasRenderingContext2D | null;
  private readonly tex: THREE.CanvasTexture;
  private name = "";
  private value = "";

  constructor(private readonly title: string, worldW: number) {
    const canvas = document.createElement("canvas");
    canvas.width = W;
    canvas.height = H;
    this.ctx = canvas.getContext("2d");
    this.tex = new THREE.CanvasTexture(canvas);
    this.tex.colorSpace = THREE.SRGBColorSpace;
    this.tex.anisotropy = 8;
    const worldH = (worldW * H) / W;
    const face = new THREE.Mesh(new THREE.PlaneGeometry(worldW, worldH),
      new THREE.MeshBasicMaterial({ map: this.tex, color: new THREE.Color(FACE_DIM, FACE_DIM, FACE_DIM) }));
    const frame = new THREE.Mesh(new THREE.BoxGeometry(worldW + FRAME * 2, worldH + FRAME * 2, 0.25),
      new THREE.MeshStandardMaterial({ color: H_.frame, roughness: 0.5, metalness: 0.5 }));
    frame.position.z = -0.14;
    this.group.add(frame, face);
    void document.fonts?.ready.then(() => this.draw());
  }

  /** Own row (name + stat). Redraws only when something changed. */
  setValue(name: string, value: number): void {
    const v = String(value);
    if (name === this.name && v === this.value) return;
    this.name = name;
    this.value = v;
    this.draw();
  }

  private draw(): void {
    const c = this.ctx;
    if (!c) return;
    c.fillStyle = H_.boardBg;
    c.fillRect(0, 0, W, H);
    c.textBaseline = "middle";
    c.textAlign = "center";
    c.fillStyle = H_.boardTitle;
    c.font = `64px ${FONT_STACK}`;
    c.fillText(this.title, W / 2, 90, W - 60);
    c.fillStyle = H_.boardMuted;
    c.font = `30px ${FONT_STACK}`;
    c.fillText(t("RANKING ONLINE — PRÓXIMAMENTE", "ONLINE RANKING — COMING SOON"), W / 2, 180, W - 60);
    this.drawSlots(c);
    this.drawOwn(c);
    this.tex.needsUpdate = true;
  }

  /** Empty placeholder lines: where the online top list will go. */
  private drawSlots(c: CanvasRenderingContext2D): void {
    c.strokeStyle = H_.boardMuted;
    c.lineWidth = 2;
    c.setLineDash([14, 12]);
    c.textAlign = "left";
    c.font = `34px ${FONT_STACK}`;
    for (let i = 0; i < SLOTS; i++) {
      const y = SLOT_Y0 + i * SLOT_H;
      c.strokeRect(40, y, W - 80, SLOT_H - 16);
      c.fillStyle = H_.boardMuted;
      c.fillText(`#${i + 1}`, 60, y + (SLOT_H - 16) / 2);
    }
    c.setLineDash([]);
  }

  private drawOwn(c: CanvasRenderingContext2D): void {
    c.fillStyle = H_.ownRow;
    c.fillRect(30, OWN_Y, W - 60, OWN_H);
    c.fillStyle = H_.ownText;
    c.textAlign = "left";
    c.font = `40px ${FONT_STACK}`;
    c.fillText(t(`TÚ · ${this.name}`, `YOU · ${this.name}`), 60, OWN_Y + OWN_H / 2, W - 300);
    c.textAlign = "right";
    c.font = `52px ${FONT_STACK}`;
    c.fillText(this.value, W - 60, OWN_Y + OWN_H / 2);
  }
}
