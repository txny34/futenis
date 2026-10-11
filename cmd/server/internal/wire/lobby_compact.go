package wire

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
	MsgRosterQ       = 0x93
	RosterQHead      = HeaderLen + 4
	RosterQStride    = 12
	cmPerMeter       = 100
	facingQuantum    = math.MaxInt16 / math.Pi
	CompactLobbyFlag = "2" // /lobby?v=2
)

func RosterQLen(n int) int { return RosterQHead + n*RosterQStride }

func quant(v, scale float64) int16 {
	return int16(math.Max(math.MinInt16, math.Min(math.MaxInt16, math.Round(v*scale))))
}

// EncodeRosterQ writes a compact ROSTER for entries into dst (len >= RosterQLen) and returns its length.
func EncodeRosterQ(dst []byte, entries []Entry) int {
	PutHeader(dst, MsgRosterQ)
	le := binary.LittleEndian
	le.PutUint32(dst[HeaderLen:], uint32(len(entries)))
	for i, e := range entries {
		b := dst[RosterQHead+i*RosterQStride:]
		le.PutUint32(b, uint32(e.ID))
		le.PutUint16(b[4:], uint16(quant(e.P.X, cmPerMeter)))
		le.PutUint16(b[6:], uint16(max(0, quant(e.P.Y, cmPerMeter))))
		le.PutUint16(b[8:], uint16(quant(e.P.Z, cmPerMeter)))
		le.PutUint16(b[10:], uint16(quant(e.P.Facing, facingQuantum)))
	}
	return RosterQLen(len(entries))
}
