import type { CharacterPose } from "../character/character";
import { EventKind, SimEvent } from "../snapshot";

// Full-body festejo / bajón after a set or the match, and lobby emotes (character/gestures.ts),
// eased in and out. Running fades it: a player chasing the next serve just plays.

const SET_S = 2.4;
const MATCH_S = 6;
const EASE_S = 0.3;
const RUN_FADE = 0.8;

export type Kind = "cheer" | "slump" | "wave" | "dance";
/** Lobby emotes by wire kind (cmd/server/internal/wire/lobby_look.go): 1 saludo, 2 festejo, 3 baile, 4 bajón. */
export const EMOTES: readonly Kind[] = ["wave", "cheer", "dance", "slump"];
export const EMOTE_S = 2.6;
interface Reaction { readonly kind: Kind; readonly seconds: number }
const SET_WIN: Reaction = { kind: "cheer", seconds: SET_S };
const SET_LOSS: Reaction = { kind: "slump", seconds: SET_S };
const MATCH_WIN: Reaction = { kind: "cheer", seconds: MATCH_S };
const MATCH_LOSS: Reaction = { kind: "slump", seconds: MATCH_S };

/** The reaction of a player of `team` to e, or null. */
export function reactionFor(e: SimEvent, team: number): Reaction | null {
  const mine = e.side === team;
  if (e.kind === EventKind.SetWon) return mine ? SET_WIN : SET_LOSS;
  if (e.kind === EventKind.GameOver) return mine ? MATCH_WIN : MATCH_LOSS;
  return null;
}

export class BodyReaction {
  private kind: Kind = "cheer";
  private left = 0;
  private total = 0;

  reset(): void { this.left = 0; }

  trigger(kind: Kind, seconds: number): void {
    this.kind = kind;
    this.left = this.total = seconds;
  }

  /** Writes pose.cheer / pose.slump for this frame (faded by pose.speed). */
  update(pose: CharacterPose, dt: number): void {
    const speed = pose.speed;
    this.left = Math.max(0, this.left - dt);
    const easeIn = Math.min(1, (this.total - this.left) / EASE_S);
    const easeOut = Math.min(1, this.left / EASE_S);
    const w = this.left > 0 ? Math.min(easeIn, easeOut) * (1 - RUN_FADE * speed) : 0;
    pose.cheer = this.kind === "cheer" ? w : 0;
    pose.slump = this.kind === "slump" ? w : 0;
    pose.wave = this.kind === "wave" ? w : 0;
    pose.dance = this.kind === "dance" ? w : 0;
  }
}
