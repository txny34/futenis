// Online spike wire protocol v1 (docs/design/online-spike.md §4). LE, 8-byte header.
export const PROTO_VERSION = 1;
export const HEADER = 8;
export const Msg = {
  Input: 0x01, Ping: 0x02, Auth: 0x03, Welcome: 0x81, Waiting: 0x82, Frame: 0x83, Pong: 0x84, PeerLeft: 0x85, Result: 0x86,
} as const;
export const INPUT_MSG_LEN = HEADER + 6 * 8;
export const PING_MSG_LEN = HEADER + 8;

export function writeHeader(v: DataView, type: number): void {
  for (let i = 0; i < HEADER; i++) v.setUint8(i, 0);
  v.setUint8(0, type);
  v.setUint8(1, PROTO_VERSION);
}

export const f64At = (v: DataView, i: number): number => v.getFloat64(HEADER + i * 8, true);
export const setF64At = (v: DataView, i: number, x: number): void => v.setFloat64(HEADER + i * 8, x, true);
