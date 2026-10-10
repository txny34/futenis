package main

import (
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
