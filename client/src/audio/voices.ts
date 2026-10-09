import { Mixer } from "./mixer";

// Procedural one-shot voices (no audio assets). Each call builds a tiny
// short-lived node chain; events are sparse so this never runs per frame.
const ATTACK_S = 0.004;
const SILENT = 0.0001;
const PAN_MAX = 1;

export interface ToneSpec { freq: number; endFreq: number; dur: number; gain: number; type: OscillatorType }
export interface NoiseSpec { filter: BiquadFilterType; freq: number; q: number; dur: number; gain: number }

function out(m: Mixer, ctx: AudioContext, pan: number, peak: number, dur: number, at: number): GainNode | null {
  const bus = m.sfx;
  if (!bus) return null;
  const g = ctx.createGain();
  g.gain.setValueAtTime(SILENT, at);
  g.gain.exponentialRampToValueAtTime(Math.max(peak, SILENT), at + ATTACK_S);
  g.gain.exponentialRampToValueAtTime(SILENT, at + dur);
  const p = ctx.createStereoPanner();
  p.pan.setValueAtTime(Math.max(-PAN_MAX, Math.min(PAN_MAX, pan)), at);
  g.connect(p).connect(bus);
  return g;
}

/** Pitched blip with an exponential pitch glide. `delay` in seconds. */
export function tone(m: Mixer, s: ToneSpec, pan = 0, delay = 0): void {
  const ctx = m.ctx;
  if (!ctx) return;
  const at = ctx.currentTime + delay;
  const g = out(m, ctx, pan, s.gain, s.dur, at);
  if (!g) return;
  const o = ctx.createOscillator();
  o.type = s.type;
  o.frequency.setValueAtTime(s.freq, at);
  o.frequency.exponentialRampToValueAtTime(s.endFreq, at + s.dur);
  o.connect(g);
  o.start(at);
  o.stop(at + s.dur);
}

/** Filtered noise burst (bounces, glass, net, swishes). */
export function noise(m: Mixer, s: NoiseSpec, pan = 0, delay = 0): void {
  const ctx = m.ctx;
  const buf = m.noise;
  if (!ctx || !buf) return;
  const at = ctx.currentTime + delay;
  const g = out(m, ctx, pan, s.gain, s.dur, at);
  if (!g) return;
  const src = ctx.createBufferSource();
  src.buffer = buf;
  const f = ctx.createBiquadFilter();
  f.type = s.filter;
  f.frequency.value = s.freq;
  f.Q.value = s.q;
  src.connect(f).connect(g);
  src.start(at, 0, s.dur);
}

/** Short ascending (or descending) arpeggio for score jingles. */
export function arpeggio(m: Mixer, freqs: readonly number[], step: number, gain: number, pan = 0): void {
  for (let i = 0; i < freqs.length; i++) {
    tone(m, { freq: freqs[i], endFreq: freqs[i], dur: step * 2, gain, type: "triangle" }, pan, i * step);
  }
}
