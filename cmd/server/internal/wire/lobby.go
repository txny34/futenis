package wire

import (
	"math"

	"github.com/txny34/futenis/sim"
)

// Shared-lobby presence protocol (same header as match.go; mirrors
// client/src/net/lobby-proto.ts). The lobby has no rules: clients report their
// own position and the server only bounds and relays it.
const (
	MsgPresence  = 0x10 // client: x, y, z, facing
	MsgName      = 0x11 // client: UTF-8 name bytes
	MsgRoster    = 0x90 // server: count, then count x (id, x, y, z, facing)
	MsgLobbyName = 0x91 // server: id, then UTF-8 name bytes
	MsgGone      = 0x92 // server: id
	MsgReport    = 0x12 // client: f64 target id

	presenceFloats = 4
	PresenceLen    = HeaderLen + presenceFloats*8
	RosterStride   = 5
	MaxNameBytes   = 64
	maxJumpY       = 3
)

var (
	LobbyHalfX = sim.DefaultConfig().LobbyHalfX
	LobbyHalfZ = sim.DefaultConfig().LobbyHalfZ
)

type Presence struct{ X, Y, Z, Facing float64 }

type Entry struct {
	ID int
	P  Presence
}

func clampTo(v, lim float64) float64 { return math.Max(-lim, math.Min(lim, v)) }

// DecodePresence parses and bounds a PRESENCE: inside the lobby, feet between
// the floor and a jump, facing wrapped to [-pi, pi].
func DecodePresence(b []byte) (Presence, error) {
	if len(b) != PresenceLen || b[0] != MsgPresence || b[1] != Version {
		return Presence{}, ErrBadMessage
	}
	return Presence{
		X: clampTo(F64(b, 0), LobbyHalfX), Y: math.Max(0, math.Min(maxJumpY, F64(b, 1))),
		Z: clampTo(F64(b, 2), LobbyHalfZ), Facing: math.Remainder(F64(b, 3), 2*math.Pi),
	}, nil
}

// IsNameMsg reports a NAME message within the byte cap.
func IsNameMsg(b []byte) bool {
	return len(b) >= HeaderLen && len(b) <= HeaderLen+MaxNameBytes && b[0] == MsgName && b[1] == Version
}

func RosterLen(n int) int { return HeaderLen + (1+n*RosterStride)*8 }

// EncodeRoster writes a ROSTER for entries into dst (len >= RosterLen) and
// returns its length.
func EncodeRoster(dst []byte, entries []Entry) int {
	PutHeader(dst, MsgRoster)
	PutF64(dst, 0, float64(len(entries)))
	for i, e := range entries {
		k := 1 + i*RosterStride
		PutF64(dst, k, float64(e.ID))
		PutF64(dst, k+1, e.P.X)
		PutF64(dst, k+2, e.P.Y)
		PutF64(dst, k+3, e.P.Z)
		PutF64(dst, k+4, e.P.Facing)
	}
	return RosterLen(len(entries))
}

// EncodeLobbyName is a PLAYER_NAME: id, then the UTF-8 name.
func EncodeLobbyName(id int, name string) []byte {
	b := make([]byte, HeaderLen+8+len(name))
	PutHeader(b, MsgLobbyName)
	PutF64(b, 0, float64(id))
	copy(b[HeaderLen+8:], name)
	return b
}

// EncodeGone is a GONE: player id left the lobby.
func EncodeGone(id int) []byte {
	b := make([]byte, HeaderLen+8)
	PutHeader(b, MsgGone)
	PutF64(b, 0, float64(id))
	return b
}

// IsReportMsg reports a REPORT (lobby moderation: f64 target id).
func IsReportMsg(b []byte) bool {
	return len(b) == HeaderLen+8 && b[0] == MsgReport && b[1] == Version
}

// EncodePresence builds a PRESENCE as a client sends it (tests, tools).
func EncodePresence(x, y, z, facing float64) []byte {
	b := make([]byte, PresenceLen)
	PutHeader(b, MsgPresence)
	PutF64(b, 0, x)
	PutF64(b, 1, y)
	PutF64(b, 2, z)
	PutF64(b, 3, facing)
	return b
}
