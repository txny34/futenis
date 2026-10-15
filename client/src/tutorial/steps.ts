import { LOB_VY_RATIO } from "../feel/shots";
import { t } from "../i18n/t";
import { HUMAN_SLOT } from "../loop";
import { EventKind, Quality, SimEvent } from "../snapshot";
import { keyHint, moveHint } from "../ui/key-hints";
import type { StepRule } from "./progress";

// Tutorial (~2 min) over the ball machine: each step completes from the sim
// event stream only (the client never decides rules, it just watches).

export interface TutorialStep extends StepRule {
  readonly title: string;
  /** Built when shown: key names follow Ajustes > Controles. */
  readonly text: () => string;
}

const mine = (e: SimEvent): boolean => e.player === HUMAN_SLOT;
const kick = (e: SimEvent): boolean => e.kind === EventKind.Kick && mine(e);
// Looser than feel/shots isLob: a soft lob from chest height launches under 7 m/s.
const lob = (e: SimEvent): boolean => kick(e) && e.vy > 0 && e.vy > Math.hypot(e.vx, e.vz) * LOB_VY_RATIO;

export const TUTORIAL_STEPS: readonly TutorialStep[] = [
  {
    title: t("Golpe", "Strike"),
    text: () => t(`Movete con ${moveHint()} y pateá la pelota con click izquierdo (RT).`, `Move with ${moveHint()} and kick the ball with left click (RT).`),
    count: 1, match: kick,
  },
  {
    title: t("Devolución", "Return"),
    text: () => t(
      "Devolvé 3 pelotas a la cancha rival. Apuntá con el mouse (stick derecho).",
      "Return 3 balls to the opponent's court. Aim with the mouse (right stick).",
    ),
    count: 3, match: (e) => e.kind === EventKind.ReturnOK,
  },
  {
    title: t("Pared", "Wall"),
    text: () => t(
      "Dejá que la pelota pique y rebote en tu vidrio, y recién ahí devolvela: la pared te da tiempo.",
      "Let the ball bounce and come off your glass, then return it: the wall buys you time.",
    ),
    count: 1, match: (e) => e.kind === EventKind.ReturnOK,
    armedBy: (e) => e.kind === EventKind.HitWall && e.side === 0,
  },
  {
    title: t("Salto", "Jump"),
    text: () => t(`Saltá con ${keyHint("jump")} (A): llegás a pelotas altas.`, `Jump with ${keyHint("jump")} (A) to reach high balls.`),
    count: 1, match: (e) => e.kind === EventKind.Jump && mine(e),
  },
  {
    title: "Dash",
    text: () => t(`Hacé un dash con ${keyHint("dash")} (LB/RB): gasta energía, mirá la barra.`, `Dash with ${keyHint("dash")} (LB/RB): costs energy, watch the bar.`),
    count: 1, match: (e) => e.kind === EventKind.Dash && mine(e),
  },
  {
    title: t("Globo", "Lob"),
    text: () => t(
      "Tirá un globo con click derecho (LT): alto y por encima del rival.",
      "Kick a lob with right click (LT): high, over the opponent.",
    ),
    count: 1, match: lob,
  },
  {
    title: "Perfect",
    text: () => t(
      "Pegale justo cuando el anillo se cierra: ¡PERFECT! es punto ganador.",
      "Hit it right when the ring closes: PERFECT! is a winning point.",
    ),
    count: 1, match: (e) => kick(e) && e.quality === Quality.Perfect,
  },
];

