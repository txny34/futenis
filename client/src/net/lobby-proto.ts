// Shared-lobby presence protocol (mirrors cmd/server/internal/wire/lobby.go). Same
// 8-byte LE header as protocol.ts.
export const LobbyMsg = {
  Presence: 0x10, Name: 0x11, Report: 0x12, Style: 0x13, Emote: 0x14,
  Roster: 0x90, PlayerName: 0x91, Gone: 0x92, RosterQ: 0x93, Look: 0x94, PlayerEmote: 0x95,
} as const;
export const PRESENCE_LEN = 8 + 4 * 8;
export const ROSTER_STRIDE = 5; // id, x, y, z, facing
export const MAX_NAME_BYTES = 64;
// ROSTER_Q (cmd/server/internal/wire/lobby_compact.go, asked for with /lobby?v=2): u32 count,
// then count x (u32 id, i16 x cm, u16 y cm, i16 z cm, i16 facing 1/32767 of pi).
export const ROSTER_Q_HEAD = 8 + 4;
export const ROSTER_Q_STRIDE = 12;
export const CM = 0.01;
export const FACING_Q = Math.PI / 32767;
