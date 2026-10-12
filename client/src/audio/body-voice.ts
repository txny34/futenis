import { EventKind, Extra, PLAYER_EXTRA, Quality, SimEvent, Snap, teamOf } from "../snapshot";
import { Mixer } from "./mixer";
import { noise, NoiseSpec } from "./voices";

// The players' own sounds (procedural, no assets): an effort grunt on every
// kick, a "¡vamos!" whoop or a sigh when a set or the match ends, and my
// footsteps at the gait's cadence. Reads events and the snapshot only.

/** A voiced burst: a buzzy source through two vowel formants (f1, f2). */
export interface VocalSpec { freq: number; endFreq: number; dur: number; gain: number; f1: number; f2: number }

const ME = 0; // my player is slot 0 and my team is 0 (online frames are rotated server-side)
const COURT_HALF_WIDTH = 5;
const PAN_SCALE = 0.7;
const OTHERS_GAIN = 0.55; // everyone else's grunts sit behind mine
const SILENT = 0.0001;

const GRUNT: VocalSpec = { freq: 170, endFreq: 110, dur: 0.13, gain: 0.32, f1: 600, f2: 1050 }; // "uf"
const GRUNT_PERFECT: VocalSpec = { freq: 220, endFreq: 150, dur: 0.17, gain: 0.4, f1: 780, f2: 1250 }; // "¡ah!"
const GRUNT_OTHER: VocalSpec = { ...GRUNT, gain: GRUNT.gain * OTHERS_GAIN };
const GRUNT_PERFECT_OTHER: VocalSpec = { ...GRUNT_PERFECT, gain: GRUNT_PERFECT.gain * OTHERS_GAIN };
const WHOOP: VocalSpec = { freq: 210, endFreq: 360, dur: 0.32, gain: 0.34, f1: 760, f2: 1300 }; // "¡vamos!"
const WHOOP_TAIL: VocalSpec = { freq: 330, endFreq: 300, dur: 0.22, gain: 0.3, f1: 650, f2: 1100 };
const SIGH: VocalSpec = { freq: 175, endFreq: 105, dur: 0.6, gain: 0.2, f1: 700, f2: 1150 };
const BREATH: NoiseSpec = { filter: "bandpass", freq: 1500, q: 0.8, dur: 0.55, gain: 0.08, attack: 0.08 };

const STEP_THUD: NoiseSpec = { filter: "lowpass", freq: 320, q: 0.8, dur: 0.07, gain: 0.22 };
const STEP_THUD_SOFT: NoiseSpec = { ...STEP_THUD, gain: STEP_THUD.gain * 0.6 };
const STALE_S = 0.25; // no new snapshot for this long (hold, drop, reconnect): no steps
const STEP_SQUEAK: NoiseSpec = { filter: "bandpass", freq: 3600, q: 6, dur: 0.05, gain: 0.05 };
const GAIT_BASE_HZ = 1.1; // mirrors character/animator.ts (one cycle = two steps)
const GAIT_SPEED_HZ = 0.9;
const SPEED_REF = 7.5; // m/s = normalized speed 1 (view/players.ts)
const STEP_MIN_SPEED = 0.12;

export function vocal(m: Mixer, s: VocalSpec, pan = 0, delay = 0): void {
  const ctx = m.ctx;
  const bus = m.sfx;
  if (!ctx || !bus) return;
  const at = ctx.currentTime + delay;
  const o = ctx.createOscillator();
  o.type = "sawtooth";
  o.frequency.setValueAtTime(s.freq, at);
  o.frequency.exponentialRampToValueAtTime(s.endFreq, at + s.dur);
  const g = ctx.createGain();
  g.gain.setValueAtTime(SILENT, at);
  g.gain.exponentialRampToValueAtTime(s.gain, at + 0.02);
  g.gain.exponentialRampToValueAtTime(SILENT, at + s.dur);
  const p = ctx.createStereoPanner();
  p.pan.setValueAtTime(Math.max(-1, Math.min(1, pan)), at);
  for (const f of [s.f1, s.f2]) {
    const bp = ctx.createBiquadFilter();
    bp.type = "bandpass";
    bp.frequency.value = f;
    bp.Q.value = 5;
    o.connect(bp).connect(g);
  }
  g.connect(p).connect(bus);
  o.start(at);
  o.stop(at + s.dur);
}

export class BodyVoice {
  private gait = 0; // steps owed (0..1 per step)
  private lastTick = -1;
  private staleS = 0;
  private cheeredTick = -1; // the match point fires SetWon and GameOver in one step: one voice
  private players = 2;

  constructor(private readonly mixer: Mixer) {}

  onEvent(e: SimEvent): void {
    const m = this.mixer;
    if (e.kind === EventKind.Kick) {
      const perfect = e.quality === Quality.Perfect;
      const mine = e.player === ME;
      vocal(m, perfect ? (mine ? GRUNT_PERFECT : GRUNT_PERFECT_OTHER) : mine ? GRUNT : GRUNT_OTHER, (e.z / COURT_HALF_WIDTH) * PAN_SCALE);
      return;
    }
    if ((e.kind !== EventKind.SetWon && e.kind !== EventKind.GameOver) || e.tick === this.cheeredTick) return;
    this.cheeredTick = e.tick;
    if (e.side === teamOf(ME, this.players)) {
      vocal(m, WHOOP);
      vocal(m, WHOOP_TAIL, 0, WHOOP.dur * 0.9);
    } else {
      vocal(m, SIGH);
      noise(m, BREATH);
    }
  }

  /** My footsteps: one per half gait cycle while running on the ground. */
  frame(snap: Float64Array, dt: number): void {
    this.players = snap[Snap.NumPlayers] | 0;
    if (dt <= 0) return; // paused or hit-stop: keep the cadence
    this.staleS = snap[Snap.Tick] === this.lastTick ? this.staleS + dt : 0;
    this.lastTick = snap[Snap.Tick];
    const e = PLAYER_EXTRA[ME];
    const speed = Math.min(1, Math.hypot(snap[e + Extra.VelX], snap[e + Extra.VelZ]) / SPEED_REF);
    if (this.staleS > STALE_S || speed < STEP_MIN_SPEED || snap[e + Extra.OnGround] === 0) { this.gait = 0; return; }
    this.gait += dt * (GAIT_BASE_HZ + GAIT_SPEED_HZ * speed) * 2;
    if (this.gait < 1) return;
    this.gait -= 1;
    noise(this.mixer, speed > 0.5 ? STEP_THUD : STEP_THUD_SOFT);
    if (speed > 0.5) noise(this.mixer, STEP_SQUEAK);
  }
}
