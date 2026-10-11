package main

import (
	"encoding/binary"
	"math"
)

// Compact ROSTER (ROSTER_Q, for clients that open /lobby?v=2): 12 bytes per
// player instead of 40. After the header: u32 count, then count x (u32 id,
// i16 x cm, u16 y cm, i16 z cm, i16 facing in 1/32767 of pi), little endian.
// Mirrors client/src/lobby/presence.ts. The lobby is bounded to tens of
// meters, so centimeters fit int16 with room to spare.
const (
	msgRosterQ       = 0x93
	rosterQHead      = headerLen + 4
	rosterQStride    = 12
	cmPerMeter       = 100
	facingQuantum    = math.MaxInt16 / math.Pi
	compactLobbyFlag = "2" // /lobby?v=2
)

func rosterQLen(n int) int { return rosterQHead + n*rosterQStride }

func quant(v, scale float64) int16 {
	return int16(math.Max(math.MinInt16, math.Min(math.MaxInt16, math.Round(v*scale))))
}

// encodeRosterQ writes a compact ROSTER for entries into dst (len >= rosterQLen) and returns its length.
func encodeRosterQ(dst []byte, entries []lobbyEntry) int {
	putHeader(dst, msgRosterQ)
	le := binary.LittleEndian
	le.PutUint32(dst[headerLen:], uint32(len(entries)))
	for i, e := range entries {
		b := dst[rosterQHead+i*rosterQStride:]
		le.PutUint32(b, uint32(e.id))
		le.PutUint16(b[4:], uint16(quant(e.p.x, cmPerMeter)))
		le.PutUint16(b[6:], uint16(max(0, quant(e.p.y, cmPerMeter))))
		le.PutUint16(b[8:], uint16(quant(e.p.z, cmPerMeter)))
		le.PutUint16(b[10:], uint16(quant(e.p.facing, facingQuantum)))
	}
	return rosterQLen(len(entries))
}
