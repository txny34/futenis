import type { Engine } from "./engine";
import type { TrainingSettings } from "./ui/training-settings";

// Mirrors the Entrenamiento scalars appended to sim/config_layout.go
// (docs/design/training.md §1), right after cfgFirstServer = 54. CHANGE
// TOGETHER with the Go layout; nothing before index 55 moves.
export const TrainCfg = {
  MachineZMin: 55, MachineZMax: 56, MachineXMin: 57, MachineXMax: 58,
  FeedZMin: 59, FeedZMax: 60, FeedXMin: 61, FeedXMax: 62,
  FeedShots: 63, FeedPowerMin: 64, FeedPowerMax: 65, FeedQualityMin: 66, FeedQualityMax: 67,
  FeedOnDemand: 68, FeedInterval: 69, FeedVolleyOnly: 70, FeedValidBounce: 71,
} as const;

/** Inclusive index span of the training block (dev panel keeps it on "Reset defaults"). */
export const CFG_TRAIN_FIRST = TrainCfg.MachineZMin;
export const CFG_TRAIN_LAST = TrainCfg.FeedValidBounce;

const PCT = 100;

/** Writes t into a Config buffer (UI units -> sim units: % -> 0..1, booleans -> 0/1). */
export function writeTrainingConfig(t: TrainingSettings, cfg: Float64Array): void {
  cfg[TrainCfg.MachineZMin] = t.machineZ.lo;
  cfg[TrainCfg.MachineZMax] = t.machineZ.hi;
  cfg[TrainCfg.MachineXMin] = t.machineX.lo;
  cfg[TrainCfg.MachineXMax] = t.machineX.hi;
  cfg[TrainCfg.FeedZMin] = t.feedZ.lo;
  cfg[TrainCfg.FeedZMax] = t.feedZ.hi;
  cfg[TrainCfg.FeedXMin] = t.feedX.lo;
  cfg[TrainCfg.FeedXMax] = t.feedX.hi;
  cfg[TrainCfg.FeedShots] = t.shots;
  cfg[TrainCfg.FeedPowerMin] = t.power.lo / PCT;
  cfg[TrainCfg.FeedPowerMax] = t.power.hi / PCT;
  cfg[TrainCfg.FeedQualityMin] = t.quality.lo / PCT;
  cfg[TrainCfg.FeedQualityMax] = t.quality.hi / PCT;
  cfg[TrainCfg.FeedOnDemand] = t.onDemand ? 1 : 0;
  cfg[TrainCfg.FeedInterval] = t.interval;
  cfg[TrainCfg.FeedVolleyOnly] = t.volleyOnly ? 1 : 0;
  cfg[TrainCfg.FeedValidBounce] = t.validBounce;
}

/** Pushes t into the live Config (between frames, after configure() and before reset()). */
export function applyTraining(engine: Engine, t: TrainingSettings): void {
  const cfg = engine.getConfig();
  writeTrainingConfig(t, cfg);
  engine.setConfig(cfg);
}
