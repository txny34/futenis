import type { Emotion } from "../character/character";
import { EventKind, Quality, SimEvent } from "../snapshot";

// Which face each player makes for a game event (visual only): a point makes
// the winners happy and the losers sad, a set or the match makes them cheer or
// sulk, a perfect contact is smug, a whiff is a surprise, set/match point is
// focus. The sim already decided all of it; this only maps events to moods.

export interface Mood { readonly emotion: Emotion; readonly holdS: number }

const POINT_WIN: Mood = { emotion: "happy", holdS: 1.4 };
const POINT_LOSS: Mood = { emotion: "sad", holdS: 1.4 };
const SET_WIN: Mood = { emotion: "cheer", holdS: 2.2 };
const SET_LOSS: Mood = { emotion: "angry", holdS: 2.2 };
const MATCH_WIN: Mood = { emotion: "cheer", holdS: 6 };
const MATCH_LOSS: Mood = { emotion: "sad", holdS: 6 };
const PERFECT: Mood = { emotion: "smug", holdS: 1.1 };
const WHIFF: Mood = { emotion: "surprised", holdS: 0.9 };
const ON_POINT: Mood = { emotion: "focus", holdS: 1.6 };

/** The mood of a player of `team` (index `player`) for event e, or null for no change. */
export function moodFor(e: SimEvent, player: number, team: number): Mood | null {
  const mine = e.side === team;
  switch (e.kind) {
    case EventKind.Point: return mine ? POINT_WIN : POINT_LOSS;
    case EventKind.SetWon: return mine ? SET_WIN : SET_LOSS;
    case EventKind.GameOver: return mine ? MATCH_WIN : MATCH_LOSS;
    case EventKind.SetPoint:
    case EventKind.MatchPoint: return mine ? ON_POINT : null;
    case EventKind.Kick: return e.player === player && e.quality === Quality.Perfect ? PERFECT : null;
    case EventKind.Whiff: return e.player === player ? WHIFF : null;
    default: return null;
  }
}
