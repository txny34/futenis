// Where a player faces and whether it side-steps (padel shuffle), from the
// snapshot velocity. Hysteresis keeps a diagonal near the threshold from
// flickering; facing turns smoothly (shortest way) at the same pace the gait
// blends, so the legs always match the body's direction. Allocation free.

const FACING_MIN_SPEED = 0.5; // m/s: below this, players face the net
const SHUFFLE_ENTER_MAX = 0.6; // normalized speed: above it a player turns and runs
const SHUFFLE_EXIT_MAX = 0.7;
const SHUFFLE_ENTER_RATIO = 1.5; // sideways (Z) over forward (X) speed
const SHUFFLE_EXIT_RATIO = 1.1;
const BLEND_HZ = 10;

export interface Steer { side: boolean; strafe: number; facing: number }

const wrap = (a: number): number => Math.atan2(Math.sin(a), Math.cos(a));

/** Updates st for velocity (vx, vz) m/s, normalized speed n, net-facing yaw `net`. */
export function steer(st: Steer, vx: number, vz: number, n: number, airborne: boolean, net: number, dt: number): void {
  const speed = Math.hypot(vx, vz);
  const ax = Math.abs(vx), az = Math.abs(vz);
  if (!airborne) { // keep the stance through a jump
    st.side = st.side
      ? speed > FACING_MIN_SPEED && n < SHUFFLE_EXIT_MAX && az > SHUFFLE_EXIT_RATIO * ax
      : speed > FACING_MIN_SPEED && n < SHUFFLE_ENTER_MAX && az > SHUFFLE_ENTER_RATIO * ax;
  }
  const k = Math.min(1, dt * BLEND_HZ);
  const dir = st.side ? Math.sign(vx * Math.cos(net) - vz * Math.sin(net)) : 0; // move . body-left
  st.strafe += (dir - st.strafe) * k;
  const target = st.side || speed <= FACING_MIN_SPEED ? net : Math.atan2(vx, vz);
  st.facing = wrap(st.facing + wrap(target - st.facing) * k);
}
