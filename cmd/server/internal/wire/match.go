// Package wire is the binary protocol of the online server (matches, the
// shared lobby, ranked): little endian, 8-byte header [type][version][6
// reserved], then the body. Mirrors client/src/net/protocol.ts and
// lobby-proto.ts; messages are only ever appended (docs/design/contact-v2.md §20).
package wire

import (
	"encoding/binary"
	"errors"
	"math"
	"time"

	"github.com/txny34/futenis/game"
	"github.com/txny34/futenis/sim"
)

// Wire protocol v1 (docs/design/online-spike.md §4): little endian, 8-byte
// header [type u8][version u8][6 reserved bytes], then float64 fields.
// Mirrors client/src/net/protocol.ts.
const (
	Version   = 1
	HeaderLen = 8

	MsgInput    = 0x01
	MsgPing     = 0x02
	MsgStyle    = 0x04 // the sender's look: StyleLen opaque bytes (0x03 is the ranked AUTH)
	MsgWelcome  = 0x81
	MsgWaiting  = 0x82
	MsgFrame    = 0x83
	MsgPong     = 0x84
	MsgPeerLeft = 0x85
	MsgSeat     = 0x87 // rejoin token (ASCII hex) after the header
	MsgPeerAway = 0x88 // f64: seconds the dropped player has to come back
	MsgPeerBack = 0x89
	MsgStyles   = 0x8A // every seat's look: MaxPlayers x StyleLen, server slot order

	InputFloats = 6
	InputLen    = HeaderLen + InputFloats*8
	PingLen     = HeaderLen + 8
	WelcomeLen  = HeaderLen + 2*8
	FrameLen    = HeaderLen + game.OutLen*8

	maxButtons = 255

	// StyleLen: a look is [present, hair, headwear, skin, hairColor, hatColor,
	// headSize, build] bytes. The server only relays them; the client
	// validates (client/src/net/style-wire.ts). present 0 = bot or empty seat.
	StyleLen     = 8
	StyleMsgLen  = HeaderLen + StyleLen
	StylesMsgLen = HeaderLen + sim.MaxPlayers*StyleLen
)

// Style is one seat's look as relayed (see StyleLen).
type Style [StyleLen]byte

var ErrBadMessage = errors.New("wire: malformed message")

func PutHeader(b []byte, typ byte) {
	clear(b[:HeaderLen])
	b[0], b[1] = typ, Version
}

func PutF64(b []byte, i int, f float64) {
	binary.LittleEndian.PutUint64(b[HeaderLen+i*8:], math.Float64bits(f))
}

// F64 reads float i of the body; NaN/Inf become 0 (never trust the client).
func F64(b []byte, i int) float64 {
	f := math.Float64frombits(binary.LittleEndian.Uint64(b[HeaderLen+i*8:]))
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return f
}

// DecodeInput parses an INPUT message into a PlayerInput (client view).
func DecodeInput(b []byte) (sim.PlayerInput, error) {
	if len(b) != InputLen || b[0] != MsgInput || b[1] != Version {
		return sim.PlayerInput{}, ErrBadMessage
	}
	return sim.PlayerInput{
		MoveX: clampUnit(F64(b, 0)), MoveZ: clampUnit(F64(b, 1)),
		TargetX: F64(b, 2), TargetZ: F64(b, 3),
		Buttons: uint8(math.Max(0, math.Min(maxButtons, F64(b, 4)))),
		Spin:    int8(clampUnit(F64(b, 5))),
	}, nil
}

// clampUnit bounds a value to [-1, 1] (move axes, spin).
func clampUnit(v float64) float64 { return math.Max(-1, math.Min(1, v)) }

// IsPing reports a well-formed PING; pong reuses its bytes with the type flipped.
func IsPing(b []byte) bool { return len(b) == PingLen && b[0] == MsgPing && b[1] == Version }

func EncodePong(dst, ping []byte) []byte {
	dst = dst[:PingLen]
	copy(dst, ping)
	dst[0] = MsgPong
	return dst
}

func EncodeWelcome(slot int) []byte {
	b := make([]byte, WelcomeLen)
	PutHeader(b, MsgWelcome)
	b[2] = byte(slot)
	PutF64(b, 0, sim.SnapLen)
	PutF64(b, 1, game.OutLen)
	return b
}

func EncodeSeat(token string) []byte {
	b := make([]byte, HeaderLen+len(token))
	PutHeader(b, MsgSeat)
	copy(b[HeaderLen:], token)
	return b
}

func EncodeAway(grace time.Duration) []byte {
	b := make([]byte, HeaderLen+8)
	PutHeader(b, MsgPeerAway)
	PutF64(b, 0, grace.Seconds())
	return b
}

func EncodeSignal(typ byte) []byte {
	b := make([]byte, HeaderLen)
	PutHeader(b, typ)
	return b
}

// EncodeFrame writes a FRAME message for out into dst (len FrameLen).
func EncodeFrame(dst []byte, out []float64) {
	PutHeader(dst, MsgFrame)
	game.EncodeFloats(dst[HeaderLen:], out)
}

// EncodeInput builds an INPUT as a client sends it (tests, tools).
func EncodeInput(vals [InputFloats]float64) []byte {
	b := make([]byte, InputLen)
	PutHeader(b, MsgInput)
	for i, v := range vals {
		PutF64(b, i, v)
	}
	return b
}

// DecodeStyle parses a STYLE message; the result is marked present.
func DecodeStyle(b []byte) (Style, bool) {
	var st Style
	if len(b) != StyleMsgLen || b[0] != MsgStyle || b[1] != Version {
		return st, false
	}
	copy(st[:], b[HeaderLen:])
	st[0] = 1
	return st, true
}

// EncodeStyles builds a STYLES message with every seat's look.
func EncodeStyles(styles [sim.MaxPlayers]Style) []byte {
	b := make([]byte, StylesMsgLen)
	PutHeader(b, MsgStyles)
	for i, st := range styles {
		copy(b[HeaderLen+i*StyleLen:], st[:])
	}
	return b
}

// EncodeStyle builds a STYLE as a client sends it (tests, tools).
func EncodeStyle(st Style) []byte {
	b := make([]byte, StyleMsgLen)
	PutHeader(b, MsgStyle)
	copy(b[HeaderLen:], st[:])
	return b
}
