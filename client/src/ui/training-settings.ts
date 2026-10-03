/** Entrenamiento (ball machine) settings: types, contract ranges, validation, localStorage. */

/** Closed range [lo, hi], lo <= hi. */
export interface Range { readonly lo: number; readonly hi: number }

/** UI units: meters for court ranges, percent (0..100) for power/quality, seconds for interval. */
export interface TrainingSettings {
  readonly machineZ: Range; // lateral, -Z = Izquierda from the player's view
  readonly machineX: Range; // depth from the net in court 1
  readonly feedZ: Range; // bounce target Z in the player's half
  readonly feedX: Range; // bounce target |X| in the player's half
  readonly shots: number; // bitmask: SHOT_DRIVE | SHOT_LOB | SHOT_SMASH
  readonly power: Range;
  readonly quality: Range;
  readonly onDemand: boolean;
  readonly interval: number;
  readonly volleyOnly: boolean;
  readonly validBounce: 1 | 2;
}

export interface Bounds { readonly min: number; readonly max: number; readonly step: number }

// Contract ranges (docs/design/training.md §1); Sanitize clamps the same way in sim.
export const LATERAL: Bounds = { min: -4.7, max: 4.7, step: 0.1 };
export const MACHINE_DEPTH: Bounds = { min: 2, max: 9.7, step: 0.1 };
export const FEED_DEPTH: Bounds = { min: 1, max: 9.7, step: 0.1 };
export const PERCENT: Bounds = { min: 0, max: 100, step: 1 };
export const INTERVAL: Bounds = { min: 0.5, max: 5, step: 0.1 };

export const SHOT_DRIVE = 1;
export const SHOT_LOB = 2;
export const SHOT_SMASH = 4;
const SHOTS_ALL = SHOT_DRIVE | SHOT_LOB | SHOT_SMASH;

export const DEFAULT_TRAINING: TrainingSettings = {
  machineZ: { lo: -3, hi: 3 },
  machineX: { lo: 8, hi: 9.5 },
  feedZ: { lo: -3.5, hi: 3.5 },
  feedX: { lo: 4, hi: 8.5 },
  shots: SHOT_DRIVE,
  power: { lo: 30, hi: 70 },
  quality: { lo: 70, hi: 100 },
  onDemand: false,
  interval: 1.5,
  volleyOnly: false,
  validBounce: 1,
};

const STORAGE_KEY = "futenis.training";
const SCHEMA_VERSION = 1;

type Rec = Record<string, unknown>;

function isRec(v: unknown): v is Rec {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

function clampNum(v: unknown, b: Bounds, def: number): number {
  return typeof v === "number" && Number.isFinite(v) ? Math.max(b.min, Math.min(b.max, v)) : def;
}

function range(v: unknown, b: Bounds, def: Range): Range {
  if (!isRec(v)) return def;
  const lo = clampNum(v.lo, b, def.lo);
  const hi = clampNum(v.hi, b, def.hi);
  return lo <= hi ? { lo, hi } : { lo: hi, hi: lo };
}

function bool(v: unknown, def: boolean): boolean {
  return typeof v === "boolean" ? v : def;
}

/** Field-by-field validation: invalid fields fall back to their default. */
export function validateTraining(raw: unknown): TrainingSettings {
  const d = DEFAULT_TRAINING;
  if (!isRec(raw)) return d;
  const shots = typeof raw.shots === "number" ? raw.shots & SHOTS_ALL : 0;
  return {
    machineZ: range(raw.machineZ, LATERAL, d.machineZ),
    machineX: range(raw.machineX, MACHINE_DEPTH, d.machineX),
    feedZ: range(raw.feedZ, LATERAL, d.feedZ),
    feedX: range(raw.feedX, FEED_DEPTH, d.feedX),
    shots: shots === 0 ? d.shots : shots,
    power: range(raw.power, PERCENT, d.power),
    quality: range(raw.quality, PERCENT, d.quality),
    onDemand: bool(raw.onDemand, d.onDemand),
    interval: clampNum(raw.interval, INTERVAL, d.interval),
    volleyOnly: bool(raw.volleyOnly, d.volleyOnly),
    validBounce: raw.validBounce === 2 ? 2 : 1,
  };
}

export function loadTraining(): TrainingSettings {
  try {
    const text = window.localStorage.getItem(STORAGE_KEY);
    if (text === null) return DEFAULT_TRAINING;
    const parsed: unknown = JSON.parse(text);
    if (!isRec(parsed) || parsed.version !== SCHEMA_VERSION) return DEFAULT_TRAINING;
    return validateTraining(parsed.data);
  } catch {
    return DEFAULT_TRAINING;
  }
}

/** Returns false if storage is unavailable (private mode, quota). */
export function saveTraining(t: TrainingSettings): boolean {
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify({ version: SCHEMA_VERSION, data: t }));
    return true;
  } catch {
    return false;
  }
}
