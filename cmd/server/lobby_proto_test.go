package main

import (
	"encoding/binary"
	"math"
	"testing"
)

func presenceMsg(x, y, z, facing float64) []byte {
	b := make([]byte, presenceMsgLen)
	putHeader(b, msgPresence)
	putF64(b, 0, x)
	putF64(b, 1, y)
	putF64(b, 2, z)
	putF64(b, 3, facing)
	return b
}

func TestDecodePresenceClampsToLobby(t *testing.T) {
	p, err := decodePresence(presenceMsg(999, -5, math.NaN(), 100))
	if err != nil {
		t.Fatal(err)
	}
	if p.x != lobbyHalfX || p.y != 0 || p.z != 0 {
		t.Fatalf("not clamped: %+v", p)
	}
	if p.facing < -math.Pi || p.facing > math.Pi {
		t.Fatalf("facing not wrapped: %v", p.facing)
	}
	if _, err := decodePresence(presenceMsg(0, 0, 0, 0)[:20]); err == nil {
		t.Fatal("short message accepted")
	}
}

func TestSanitizeName(t *testing.T) {
	for in, want := range map[string]string{
		"  txny44 ":                  "txny44",
		"a\x00b\nc\u202ed":           "abcd", // control and bidi-override runes dropped
		"ñandú ⚽":                    "ñandú ⚽",
		"abcdefghijklmnopqrstuvwxyz": "abcdefghijklmnop",
		"\xff\xfe":                   "",
	} {
		if got := sanitizeName(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestEncodeRoster(t *testing.T) {
	dst := make([]byte, rosterMsgLen(2))
	n := encodeRoster(dst, []lobbyEntry{{id: 7, p: presence{x: 1, y: 0, z: -2, facing: 0.5}}, {id: 9}})
	if n != rosterMsgLen(2) || dst[0] != msgRoster || f64(dst, 0) != 2 || f64(dst, 1) != 7 || f64(dst, 4) != -2 {
		t.Fatalf("bad roster: n=%d", n)
	}
}

func TestSanitizeNameDropsCombiningMarks(t *testing.T) {
	if got := sanitizeName("z̶̶̶a⃝"); got != "za" {
		t.Fatalf("got %q: stacked combining marks (zalgo) kept", got)
	}
}

func TestEncodeRosterQ(t *testing.T) {
	dst := make([]byte, rosterQLen(2))
	n := encodeRosterQ(dst, []lobbyEntry{{id: 70000, p: presence{x: -12.345, y: 1.5, z: 3.21, facing: -math.Pi}}, {id: 9}})
	le := binary.LittleEndian
	b := dst[rosterQHead:]
	if n != rosterQLen(2) || n != headerLen+4+24 || dst[0] != msgRosterQ || le.Uint32(dst[headerLen:]) != 2 {
		t.Fatalf("len %d head %#x count %d", n, dst[0], le.Uint32(dst[headerLen:]))
	}
	if le.Uint32(b) != 70000 || int16(le.Uint16(b[4:])) != -1235 || le.Uint16(b[6:]) != 150 ||
		int16(le.Uint16(b[8:])) != 321 || int16(le.Uint16(b[10:])) != -math.MaxInt16 {
		t.Fatalf("entry %v", b[:rosterQStride])
	}
	if got := rosterQLen(47); got*3 > rosterMsgLen(47) {
		t.Fatalf("compact roster %d bytes is not ~3x smaller than %d", got, rosterMsgLen(47))
	}
}
