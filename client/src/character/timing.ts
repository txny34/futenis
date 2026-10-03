// Normalized strike clip timing shared by poses, IK and squash.

/** Strike timeline: impact happens at IMPACT_T of the normalized clip. */
export const IMPACT_T = 0.3;
const IMPACT_SWING = 0.15;
const RECOVER_START = 0.75;
const IK_HALF_WIDTH = 0.18;

/** Swing value for strike time t (fast ease-in swing, eased follow-through). */
export function strikeSwing(t: number): number {
  if (t < IMPACT_T) {
    const u = t / IMPACT_T;
    return -1 + (1 + IMPACT_SWING) * u * u;
  }
  const u = (t - IMPACT_T) / (1 - IMPACT_T);
  return IMPACT_SWING + (1 - IMPACT_SWING) * Math.sin(Math.min(u * 2, 1) * Math.PI * 0.5);
}

/** Pose weight: full until recovery, then fades back to locomotion. */
export function strikeWeight(t: number): number {
  if (t <= 0 || t >= 1) return 0;
  return t < RECOVER_START ? 1 : 1 - (t - RECOVER_START) / (1 - RECOVER_START);
}

/** Bell (0..1..0) centred on impact; drives IK weight. */
export function impactBell(t: number, halfWidth = IK_HALF_WIDTH): number {
  const x = (t - IMPACT_T) / halfWidth;
  return Math.abs(x) >= 1 ? 0 : 0.5 + 0.5 * Math.cos(x * Math.PI);
}

const LIFT_START = 0.05;
const LIFT_SPAN = 0.5;

/** Jump/rise curve for headers: 0 -> 1 at impact -> 0. */
export function liftCurve(t: number): number {
  const u = (t - LIFT_START) / LIFT_SPAN;
  return u <= 0 || u >= 1 ? 0 : Math.sin(u * Math.PI);
}
