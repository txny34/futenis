package main

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/txny34/futenis/sim"
)

// Shared-lobby presence protocol (same header as proto.go; mirrors
// client/src/net/lobby-proto.ts). The lobby has no rules: clients report their
// own position and the server only bounds and relays it.
const (
	msgPresence  = 0x10 // client: x, y, z, facing
	msgName      = 0x11 // client: UTF-8 name bytes
	msgRoster    = 0x90 // server: count, then count x (id, x, y, z, facing)
	msgLobbyName = 0x91 // server: id, then UTF-8 name bytes
	msgGone      = 0x92 // server: id

	presenceFloats = 4
	presenceMsgLen = headerLen + presenceFloats*8
	rosterStride   = 5
	maxNameBytes   = 64
	maxNameRunes   = 16
	maxJumpY       = 3
)

var (
	lobbyHalfX = sim.DefaultConfig().LobbyHalfX
	lobbyHalfZ = sim.DefaultConfig().LobbyHalfZ
)

type presence struct{ x, y, z, facing float64 }

type lobbyEntry struct {
	id int
	p  presence
}

func clampTo(v, lim float64) float64 { return math.Max(-lim, math.Min(lim, v)) }

// decodePresence parses and bounds a PRESENCE: inside the lobby, feet between
// the floor and a jump, facing wrapped to [-pi, pi].
func decodePresence(b []byte) (presence, error) {
	if len(b) != presenceMsgLen || b[0] != msgPresence || b[1] != protoVersion {
		return presence{}, errBadMessage
	}
	return presence{
		x: clampTo(f64(b, 0), lobbyHalfX), y: math.Max(0, math.Min(maxJumpY, f64(b, 1))),
		z: clampTo(f64(b, 2), lobbyHalfZ), facing: math.Remainder(f64(b, 3), 2*math.Pi),
	}, nil
}

// isNameMsg reports a NAME message within the byte cap.
func isNameMsg(b []byte) bool {
	return len(b) >= headerLen && len(b) <= headerLen+maxNameBytes && b[0] == msgName && b[1] == protoVersion
}

// sanitizeName keeps printable runes only (no control, format/bidi overrides
// or combining marks, so no "zalgo" stacks), trims spaces and caps the length.
// Invalid UTF-8 is dropped.
func sanitizeName(s string) string {
	var b strings.Builder
	n := 0
	for _, r := range strings.TrimSpace(s) {
		if r == utf8.RuneError || !unicode.IsPrint(r) || unicode.In(r, unicode.Cf, unicode.Mn, unicode.Me) {
			continue
		}
		if n == maxNameRunes {
			break
		}
		b.WriteRune(r)
		n++
	}
	return strings.TrimSpace(b.String())
}

func rosterMsgLen(n int) int { return headerLen + (1+n*rosterStride)*8 }

// encodeRoster writes a ROSTER for entries into dst (len >= rosterMsgLen) and
// returns its length.
func encodeRoster(dst []byte, entries []lobbyEntry) int {
	putHeader(dst, msgRoster)
	putF64(dst, 0, float64(len(entries)))
	for i, e := range entries {
		k := 1 + i*rosterStride
		putF64(dst, k, float64(e.id))
		putF64(dst, k+1, e.p.x)
		putF64(dst, k+2, e.p.y)
		putF64(dst, k+3, e.p.z)
		putF64(dst, k+4, e.p.facing)
	}
	return rosterMsgLen(len(entries))
}

func encodeLobbyName(id int, name string) []byte {
	b := make([]byte, headerLen+8+len(name))
	putHeader(b, msgLobbyName)
	putF64(b, 0, float64(id))
	copy(b[headerLen+8:], name)
	return b
}

func encodeGone(id int) []byte {
	b := make([]byte, headerLen+8)
	putHeader(b, msgGone)
	putF64(b, 0, float64(id))
	return b
}
