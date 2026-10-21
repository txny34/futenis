package wire

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestEncodeTapeChunks(t *testing.T) {
	gz := bytes.Repeat([]byte{7}, 2*TapeChunk+5)
	msgs := EncodeTapeChunks(gz)
	if len(msgs) != 3 {
		t.Fatalf("%d chunks", len(msgs))
	}
	var back []byte
	for i, m := range msgs {
		if m[0] != MsgTape || m[1] != Version || int(binary.LittleEndian.Uint16(m[2:])) != i || binary.LittleEndian.Uint16(m[4:]) != 3 {
			t.Fatalf("chunk %d header % x", i, m[:HeaderLen])
		}
		back = append(back, m[HeaderLen:]...)
	}
	if !bytes.Equal(back, gz) {
		t.Fatal("reassembly differs")
	}
	if n := len(EncodeTapeChunks(nil)); n != 1 {
		t.Fatalf("empty tape gave %d chunks", n)
	}
}
