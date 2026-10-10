import { EventKind, Quality, SimEvent } from "../snapshot";
import { Mixer, Volumes } from "./mixer";
import { arpeggio, noise, NoiseSpec, tone, ToneSpec } from "./voices";

// Event -> sound mapping. Pure reader of the sim event stream (never decides
// rules). Camera sits behind team 0 looking +X, so +Z is screen right.
const COURT_HALF_WIDTH = 5;
const PAN_SCALE = 0.7;
const JINGLE_STEP_S = 0.09;
const JINGLE_GAIN = 0.18;

const KICK_GOOD: ToneSpec = { freq: 150, endFreq: 55, dur: 0.14, gain: 0.7, type: "sine" };
const KICK: Readonly<Record<number, ToneSpec | undefined>> = {
  [Quality.Perfect]: { freq: 180, endFreq: 60, dur: 0.18, gain: 0.9, type: "sine" },
  [Quality.Good]: KICK_GOOD,
  [Quality.Forced]: { freq: 110, endFreq: 50, dur: 0.1, gain: 0.45, type: "sine" },
};
const KICK_SNAP: NoiseSpec = { filter: "bandpass", freq: 2400, q: 1.2, dur: 0.05, gain: 0.35 };
const PERFECT_RING: ToneSpec = { freq: 1320, endFreq: 1320, dur: 0.35, gain: 0.12, type: "triangle" };
const BOUNCE: ToneSpec = { freq: 140, endFreq: 70, dur: 0.07, gain: 0.35, type: "sine" };
const WALL_KNOCK: NoiseSpec = { filter: "bandpass", freq: 1700, q: 3, dur: 0.12, gain: 0.4 };
const WALL_BODY: ToneSpec = { freq: 420, endFreq: 300, dur: 0.12, gain: 0.25, type: "triangle" };
const NET: NoiseSpec = { filter: "lowpass", freq: 900, q: 0.7, dur: 0.22, gain: 0.35 };
const SWISH: NoiseSpec = { filter: "highpass", freq: 3000, q: 0.7, dur: 0.12, gain: 0.18 };
const DASH: NoiseSpec = { filter: "bandpass", freq: 1200, q: 0.8, dur: 0.16, gain: 0.2 };
const JUMP: ToneSpec = { freq: 220, endFreq: 330, dur: 0.08, gain: 0.12, type: "sine" };
const BODY: ToneSpec = { freq: 90, endFreq: 50, dur: 0.1, gain: 0.4, type: "sine" };
const FEED: NoiseSpec = { filter: "lowpass", freq: 600, q: 2, dur: 0.1, gain: 0.45 };
const ROAR: NoiseSpec = { filter: "bandpass", freq: 700, q: 0.6, dur: 0.95, gain: 0.3, attack: 0.3 };
const WHISTLE: ToneSpec = { freq: 2100, endFreq: 1900, dur: 0.28, gain: 0.12, type: "square" };
const ALERT: ToneSpec = { freq: 880, endFreq: 880, dur: 0.18, gain: 0.12, type: "triangle" };
const RETURN_OK: ToneSpec = { freq: 660, endFreq: 990, dur: 0.12, gain: 0.14, type: "triangle" };
const RETURN_FAIL: ToneSpec = { freq: 330, endFreq: 180, dur: 0.22, gain: 0.14, type: "triangle" };

// Ranked: one step can carry Point + GameWon + SetWon + MatchWon; only the
// biggest jingle of the batch plays (flushed in a microtask).
const JINGLES: readonly (readonly number[])[] = [
  [],
  [523, 659], // point / ace
  [523, 659, 784], // game
  [523, 659, 784, 1047], // set
  [392, 523, 659, 784, 1047], // match
];
const JINGLE_RANK: Readonly<Record<number, number | undefined>> = {
  [EventKind.Point]: 1, [EventKind.Ace]: 1, [EventKind.GameWon]: 2, [EventKind.SetWon]: 3,
  [EventKind.MatchWon]: 4, [EventKind.GameOver]: 4,
};

export class Sound {
  private readonly mixer = new Mixer();
  private jingle = 0;
  private readonly flushJingle = (): void => {
    arpeggio(this.mixer, JINGLES[this.jingle], JINGLE_STEP_S, JINGLE_GAIN);
    this.jingle = 0;
  };

  setVolumes(v: Readonly<Volumes>): void {
    this.mixer.setVolumes(v);
  }

  /** Crowd roar: a swelling band of noise, louder for bigger moments (0..1). */
  cheer(level: number): void {
    const l = Math.max(0, Math.min(1, level));
    noise(this.mixer, { ...ROAR, gain: ROAR.gain * (0.5 + l / 2) });
  }

  /** Feed every non-lobby sim event in order. */
  onEvent(e: SimEvent): void {
    const pan = (e.z / COURT_HALF_WIDTH) * PAN_SCALE;
    if (this.contact(e, pan)) return;
    this.call(e);
  }

  /** Ball / body sounds, panned by court Z. Returns true when handled. */
  private contact(e: SimEvent, pan: number): boolean {
    const m = this.mixer;
    switch (e.kind) {
      case EventKind.Kick:
        tone(m, KICK[e.quality] ?? KICK_GOOD, pan);
        noise(m, KICK_SNAP, pan);
        if (e.quality === Quality.Perfect) tone(m, PERFECT_RING, pan);
        return true;
      case EventKind.BounceFloor: tone(m, BOUNCE, pan); return true;
      case EventKind.HitWall: noise(m, WALL_KNOCK, pan); tone(m, WALL_BODY, pan); return true;
      case EventKind.HitNet: noise(m, NET, pan); return true;
      case EventKind.Whiff: noise(m, SWISH, pan); return true;
      case EventKind.Dash: noise(m, DASH, pan); return true;
      case EventKind.Jump: tone(m, JUMP, pan); return true;
      case EventKind.BodyHit: tone(m, BODY, pan); return true;
      case EventKind.Feed: noise(m, FEED, pan); return true;
      default: return false;
    }
  }

  /** Referee calls and score jingles (centered). */
  private call(e: SimEvent): void {
    const m = this.mixer;
    const rank = JINGLE_RANK[e.kind] ?? 0;
    if (rank > 0) {
      if (this.jingle === 0) queueMicrotask(this.flushJingle);
      this.jingle = Math.max(this.jingle, rank);
      return;
    }
    switch (e.kind) {
      case EventKind.Out:
      case EventKind.ServeFault:
      case EventKind.Let: tone(m, WHISTLE); break;
      case EventKind.GamePoint:
      case EventKind.BreakPoint: tone(m, ALERT); break;
      case EventKind.ReturnOK: tone(m, RETURN_OK); break;
      case EventKind.ReturnFail: tone(m, RETURN_FAIL); break;
      default: break;
    }
  }
}
