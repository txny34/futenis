package main

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
	protoVersion = 1
	headerLen    = 8

	msgInput    = 0x01
	msgPing     = 0x02
	msgWelcome  = 0x81
	msgWaiting  = 0x82
	msgFrame    = 0x83
	msgPong     = 0x84
	msgPeerLeft = 0x85
	msgSeat     = 0x87 // rejoin token (ASCII hex) after the header
	msgPeerAway = 0x88 // f64: seconds the dropped player has to come back
	msgPeerBack = 0x89

	inputFloats = 6
	inputMsgLen = headerLen + inputFloats*8
	pingMsgLen  = headerLen + 8
	welcomeLen  = headerLen + 2*8
	frameMsgLen = headerLen + game.OutLen*8

	maxButtons = 255
)

var errBadMessage = errors.New("server: malformed message")

func putHeader(b []byte, typ byte) {
	clear(b[:headerLen])
	b[0], b[1] = typ, protoVersion
}

func putF64(b []byte, i int, f float64) {
	binary.LittleEndian.PutUint64(b[headerLen+i*8:], math.Float64bits(f))
}

// f64 reads float i of the body; NaN/Inf become 0 (never trust the client).
func f64(b []byte, i int) float64 {
	f := math.Float64frombits(binary.LittleEndian.Uint64(b[headerLen+i*8:]))
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return f
}

// decodeInput parses an INPUT message into a PlayerInput (client view).
func decodeInput(b []byte) (sim.PlayerInput, error) {
	if len(b) != inputMsgLen || b[0] != msgInput || b[1] != protoVersion {
		return sim.PlayerInput{}, errBadMessage
	}
	return sim.PlayerInput{
		MoveX: clampUnit(f64(b, 0)), MoveZ: clampUnit(f64(b, 1)),
		TargetX: f64(b, 2), TargetZ: f64(b, 3),
		Buttons: uint8(math.Max(0, math.Min(maxButtons, f64(b, 4)))),
		Spin:    int8(clampUnit(f64(b, 5))),
	}, nil
}

// clampUnit bounds a value to [-1, 1] (move axes, spin).
func clampUnit(v float64) float64 { return math.Max(-1, math.Min(1, v)) }

// isPing reports a well-formed PING; pong reuses its bytes with the type flipped.
func isPing(b []byte) bool { return len(b) == pingMsgLen && b[0] == msgPing && b[1] == protoVersion }

func encodePong(dst, ping []byte) []byte {
	dst = dst[:pingMsgLen]
	copy(dst, ping)
	dst[0] = msgPong
	return dst
}

func encodeWelcome(slot int) []byte {
	b := make([]byte, welcomeLen)
	putHeader(b, msgWelcome)
	b[2] = byte(slot)
	putF64(b, 0, sim.SnapLen)
	putF64(b, 1, game.OutLen)
	return b
}

func encodeSeat(token string) []byte {
	b := make([]byte, headerLen+len(token))
	putHeader(b, msgSeat)
	copy(b[headerLen:], token)
	return b
}

func encodeAway(grace time.Duration) []byte {
	b := make([]byte, headerLen+8)
	putHeader(b, msgPeerAway)
	putF64(b, 0, grace.Seconds())
	return b
}

func encodeSignal(typ byte) []byte {
	b := make([]byte, headerLen)
	putHeader(b, typ)
	return b
}

// encodeFrame writes a FRAME message for out into dst (len frameMsgLen).
func encodeFrame(dst []byte, out []float64) {
	putHeader(dst, msgFrame)
	game.EncodeFloats(dst[headerLen:], out)
}
