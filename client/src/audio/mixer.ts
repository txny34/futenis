// Web Audio graph: master -> destination, sfx/music buses -> master.
// The AudioContext is created on the first user gesture (browser autoplay
// policy) and suspended while the tab is hidden.
const NOISE_S = 1;
const PERCENT = 100;
const RAMP_S = 0.02;

export interface Volumes { master: number; sfx: number; music: number }

export class Mixer {
  private ctxRef: AudioContext | null = null;
  private master: GainNode | null = null;
  private sfxBus: GainNode | null = null;
  private musicBus: GainNode | null = null;
  private noiseBuf: AudioBuffer | null = null;
  private disabled = false;
  private vol: Volumes = { master: 80, sfx: 80, music: 60 };

  constructor() {
    const unlock = (): void => this.unlock();
    window.addEventListener("pointerdown", unlock, { capture: true });
    window.addEventListener("keydown", unlock, { capture: true });
    document.addEventListener("visibilitychange", () => this.onVisibility());
  }

  /** Live context, or null before the first gesture / when unsupported. */
  get ctx(): AudioContext | null {
    return this.ctxRef && this.ctxRef.state === "running" ? this.ctxRef : null;
  }

  get sfx(): AudioNode | null { return this.sfxBus; }

  /** 1 s of white noise, shared by every noise voice. */
  get noise(): AudioBuffer | null { return this.noiseBuf; }

  setVolumes(v: Readonly<Volumes>): void {
    this.vol = { master: v.master, sfx: v.sfx, music: v.music };
    this.applyVolumes();
  }

  private unlock(): void {
    if (this.disabled || document.visibilityState === "hidden" || this.ctxRef?.state === "running") return;
    if (!this.ctxRef) this.build();
    if (this.ctxRef && this.ctxRef.state === "suspended") void this.ctxRef.resume().catch(() => undefined);
  }

  private build(): void {
    let ctx: AudioContext;
    try {
      ctx = new AudioContext();
    } catch {
      this.disabled = true; // unsupported / context limit: play silently, never retry
      return;
    }
    this.master = ctx.createGain();
    this.master.connect(ctx.destination);
    this.sfxBus = ctx.createGain();
    this.sfxBus.connect(this.master);
    this.musicBus = ctx.createGain();
    this.musicBus.connect(this.master);
    this.noiseBuf = whiteNoise(ctx);
    this.ctxRef = ctx;
    this.applyVolumes(0);
  }

  /** `ramp` 0 = set immediately (first build: no full-volume blip). */
  private applyVolumes(ramp = RAMP_S): void {
    const ctx = this.ctxRef;
    if (!ctx || !this.master || !this.sfxBus || !this.musicBus) return;
    const t = ctx.currentTime;
    setGain(this.master, this.vol.master / PERCENT, t, ramp);
    setGain(this.sfxBus, this.vol.sfx / PERCENT, t, ramp);
    setGain(this.musicBus, this.vol.music / PERCENT, t, ramp);
  }

  private onVisibility(): void {
    const ctx = this.ctxRef;
    if (!ctx) return;
    // Resume on visible too: gamepad presses are not user gestures, and
    // sticky activation lets resume() through without a new one.
    if (document.visibilityState === "hidden") void ctx.suspend().catch(() => undefined);
    else void ctx.resume().catch(() => undefined);
  }
}

function setGain(g: GainNode, v: number, t: number, ramp: number): void {
  if (ramp > 0) g.gain.setTargetAtTime(v, t, ramp);
  else g.gain.setValueAtTime(v, t);
}

function whiteNoise(ctx: AudioContext): AudioBuffer {
  const buf = ctx.createBuffer(1, Math.floor(ctx.sampleRate * NOISE_S), ctx.sampleRate);
  const data = buf.getChannelData(0);
  let seed = 0x9e3779b9; // deterministic xorshift: no Math.random needed
  for (let i = 0; i < data.length; i++) {
    seed ^= seed << 13; seed ^= seed >>> 17; seed ^= seed << 5;
    data[i] = ((seed >>> 0) / 0xffffffff) * 2 - 1;
  }
  return buf;
}
