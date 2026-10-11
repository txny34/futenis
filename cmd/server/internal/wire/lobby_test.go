package wire

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestDecodePresenceClampsToLobby(t *testing.T) {
	p, err := DecodePresence(EncodePresence(999, -5, math.NaN(), 100))
	if err != nil {
		t.Fatal(err)
	}
	if p.X != LobbyHalfX || p.Y != 0 || p.Z != 0 {
		t.Fatalf("not clamped: %+v", p)
	}
	if p.Facing < -math.Pi || p.Facing > math.Pi {
		t.Fatalf("facing not wrapped: %v", p.Facing)
	}
	if _, err := DecodePresence(EncodePresence(0, 0, 0, 0)[:20]); err == nil {
		t.Fatal("short message accepted")
	}
}

func TestEncodeRoster(t *testing.T) {
	dst := make([]byte, RosterLen(2))
	n := EncodeRoster(dst, []Entry{{ID: 7, P: Presence{X: 1, Y: 0, Z: -2, Facing: 0.5}}, {ID: 9}})
	if n != RosterLen(2) || dst[0] != MsgRoster || F64(dst, 0) != 2 || F64(dst, 1) != 7 || F64(dst, 4) != -2 {
		t.Fatalf("bad roster: n=%d", n)
	}
}

func TestEncodeRosterQ(t *testing.T) {
	dst := make([]byte, RosterQLen(2))
	n := EncodeRosterQ(dst, []Entry{{ID: 70000, P: Presence{X: -12.345, Y: 1.5, Z: 3.21, Facing: -math.Pi}}, {ID: 9}})
	le := binary.LittleEndian
	b := dst[RosterQHead:]
	if n != RosterQLen(2) || n != HeaderLen+4+24 || dst[0] != MsgRosterQ || le.Uint32(dst[HeaderLen:]) != 2 {
		t.Fatalf("len %d head %#x count %d", n, dst[0], le.Uint32(dst[HeaderLen:]))
	}
	if le.Uint32(b) != 70000 || int16(le.Uint16(b[4:])) != -1235 || le.Uint16(b[6:]) != 150 ||
		int16(le.Uint16(b[8:])) != 321 || int16(le.Uint16(b[10:])) != -math.MaxInt16 {
		t.Fatalf("entry %v", b[:RosterQStride])
	}
	if got := RosterQLen(47); got*3 > RosterLen(47) {
		t.Fatalf("compact roster %d bytes is not ~3x smaller than %d", got, RosterLen(47))
	}
}
