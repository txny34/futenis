// Shared-lobby presence protocol (mirrors cmd/server/lobby_proto.go). Same
// 8-byte LE header as protocol.ts.
export const LobbyMsg = { Presence: 0x10, Name: 0x11, Roster: 0x90, PlayerName: 0x91, Gone: 0x92 } as const;
export const PRESENCE_LEN = 8 + 4 * 8;
export const ROSTER_STRIDE = 5; // id, x, y, z, facing
export const MAX_NAME_BYTES = 64;
