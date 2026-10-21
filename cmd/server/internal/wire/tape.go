package wire

import "encoding/binary"

// TAPE (server → client): the finished match's replay tape (game/tape.go),
// gzipped, in chunks so no single write outlasts the send timeout. Header
// bytes 2..3 = chunk index, 4..5 = chunk count (u16 LE); the client
// reassembles them (client/src/net/tape-chunks.ts).
const (
	MsgTape   = 0x8B
	TapeChunk = 32 * 1024
)

// EncodeTapeChunks splits gz into TAPE messages (at least one).
func EncodeTapeChunks(gz []byte) [][]byte {
	count := (len(gz) + TapeChunk - 1) / TapeChunk
	if count == 0 {
		count = 1
	}
	out := make([][]byte, count)
	for i := range out {
		lo := i * TapeChunk
		hi := min(lo+TapeChunk, len(gz))
		b := make([]byte, HeaderLen+hi-lo)
		PutHeader(b, MsgTape)
		binary.LittleEndian.PutUint16(b[2:], uint16(i))
		binary.LittleEndian.PutUint16(b[4:], uint16(count))
		copy(b[HeaderLen:], gz[lo:hi])
		out[i] = b
	}
	return out
}
