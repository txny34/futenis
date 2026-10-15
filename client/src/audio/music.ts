import type { Mixer } from "./mixer";

// Procedural background music (no assets): a neon synthwave loop in A minor,
// i–VI–III–VII. Lobby = pads, bass and a soft arp; match = adds kick and hats
// and sits lower under the SFX. Notes are scheduled ahead on a timer, never per frame.
export type Mood = "off" | "lobby" | "match";
type Playing = Exclude<Mood, "off">;

const LOOKAHEAD_S = 0.15;
const START_S = 0.05;
const TICK_MS = 40;
const STEPS = 16; // sixteenths per bar, one chord per bar
const FADE_TC_S = 0.4;
const SILENT = 0.0001;
const BPM: Readonly<Record<Playing, number>> = { lobby: 88, match: 108 };
const LEVEL: Readonly<Record<Playing, number>> = { lobby: 0.55, match: 0.35 };
const ARP_GAIN: Readonly<Record<Playing, number>> = { lobby: 0.035, match: 0.05 };
// [bass root, then the triad an octave up] in Hz: Am, F, C, G.
const CHORDS: readonly (readonly number[])[] = [
  [110.0, 220.0, 261.63, 329.63],
  [87.31, 174.61, 220.0, 261.63],
  [130.81, 261.63, 329.63, 392.0],
  [98.0, 196.0, 246.94, 293.66],
];
const ARP = [1, 2, 3, 2] as const;
const PAD_CUTOFF_HZ = 900;
const HAT_HZ = 7000;

export class Music {
  private mood: Mood = "off";
  private bus: GainNode | null = null; // mood fader -> mixer music bus
  private next = 0; // ctx time of the next sixteenth
  private step = 0;
  private timer = 0;

  constructor(private readonly m: Mixer) {}

  set(mood: Mood): void {
    if (mood === this.mood) return;
    this.mood = mood;
    const ctx = this.m.ctx;
    if (ctx && this.bus) this.bus.gain.setTargetAtTime(mood === "off" ? SILENT : LEVEL[mood], ctx.currentTime, FADE_TC_S);
    if (mood !== "off" && !this.timer) this.timer = window.setInterval(this.tick, TICK_MS);
  }

  private readonly tick = (): void => {
    const ctx = this.m.ctx;
    const out = this.m.music;
    if (this.mood === "off") {
      window.clearInterval(this.timer);
      this.timer = 0;
      return;
    }
    if (!ctx || !out) { this.next = 0; return; } // before the first gesture or hidden tab: pick up later
    const bus = this.bus ?? this.fader(ctx, out);
    if (this.next < ctx.currentTime) this.next = ctx.currentTime + START_S;
    const mood = this.mood;
    const stepS = 60 / BPM[mood] / 4;
    while (this.next < ctx.currentTime + LOOKAHEAD_S) {
      this.play(ctx, bus, mood, this.next, stepS);
      this.next += stepS;
      this.step = (this.step + 1) % (STEPS * CHORDS.length);
    }
  };

  private fader(ctx: AudioContext, out: AudioNode): GainNode {
    const g = ctx.createGain();
    g.gain.setValueAtTime(SILENT, ctx.currentTime);
    if (this.mood !== "off") g.gain.setTargetAtTime(LEVEL[this.mood], ctx.currentTime, FADE_TC_S);
    g.connect(out);
    this.bus = g;
    return g;
  }

  private play(ctx: AudioContext, bus: AudioNode, mood: Playing, at: number, stepS: number): void {
    const chord = CHORDS[Math.floor(this.step / STEPS)];
    const s = this.step % STEPS;
    if (s === 0) pad(ctx, bus, chord, at, stepS * STEPS);
    if (s === 0 || s === 10 || (mood === "match" && s === 8)) note(ctx, bus, "triangle", chord[0], at, stepS * 3, 0.22);
    if (s % 2 === 0) note(ctx, bus, "square", chord[ARP[(s / 2) % ARP.length]] * 2, at, stepS * 1.5, ARP_GAIN[mood]);
    if (mood !== "match") return;
    if (s % 4 === 0) kick(ctx, bus, at);
    const noise = this.m.noise;
    if (s % 4 === 2 && noise) hat(ctx, bus, noise, at);
  }
}

function env(ctx: AudioContext, bus: AudioNode, at: number, dur: number, peak: number, attack: number): GainNode {
  const g = ctx.createGain();
  g.gain.setValueAtTime(SILENT, at);
  g.gain.exponentialRampToValueAtTime(peak, at + attack);
  g.gain.exponentialRampToValueAtTime(SILENT, at + dur);
  g.connect(bus);
  return g;
}

function note(ctx: AudioContext, bus: AudioNode, type: OscillatorType, freq: number, at: number, dur: number, gain: number): void {
  const o = ctx.createOscillator();
  o.type = type;
  o.frequency.value = freq;
  o.connect(env(ctx, bus, at, dur, gain, 0.01));
  o.start(at);
  o.stop(at + dur);
}

function pad(ctx: AudioContext, bus: AudioNode, chord: readonly number[], at: number, dur: number): void {
  const f = ctx.createBiquadFilter();
  f.type = "lowpass";
  f.frequency.value = PAD_CUTOFF_HZ;
  f.connect(env(ctx, bus, at, dur, 0.06, dur * 0.3));
  for (let i = 1; i < chord.length; i++) {
    const o = ctx.createOscillator();
    o.type = "sawtooth";
    o.frequency.value = chord[i];
    o.detune.value = (i - 2) * 6; // slight spread: wider pad
    o.connect(f);
    o.start(at);
    o.stop(at + dur);
  }
}

function kick(ctx: AudioContext, bus: AudioNode, at: number): void {
  const o = ctx.createOscillator();
  o.frequency.setValueAtTime(120, at);
  o.frequency.exponentialRampToValueAtTime(45, at + 0.15);
  o.connect(env(ctx, bus, at, 0.25, 0.5, 0.005));
  o.start(at);
  o.stop(at + 0.25);
}

function hat(ctx: AudioContext, bus: AudioNode, noise: AudioBuffer, at: number): void {
  const src = ctx.createBufferSource();
  src.buffer = noise;
  const f = ctx.createBiquadFilter();
  f.type = "highpass";
  f.frequency.value = HAT_HZ;
  src.connect(f).connect(env(ctx, bus, at, 0.04, 0.06, 0.002));
  src.start(at, 0, 0.04);
}
