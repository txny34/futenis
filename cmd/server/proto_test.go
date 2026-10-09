package main

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/txny34/futenis/game"
	"github.com/txny34/futenis/sim"
)

func inputMsg(vals [inputFloats]float64) []byte {
	b := make([]byte, inputMsgLen)
	putHeader(b, msgInput)
	for i, v := range vals {
		putF64(b, i, v)
	}
	return b
}

func TestDecodeInput(t *testing.T) {
	got, err := decodeInput(inputMsg([inputFloats]float64{0.5, -0.25, 3, -2, 9, -1}))
	if err != nil {
		t.Fatal(err)
	}
	want := sim.PlayerInput{MoveX: 0.5, MoveZ: -0.25, TargetX: 3, TargetZ: -2, Buttons: 9, Spin: -1}
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestDecodeInputSanitizes(t *testing.T) {
	got, err := decodeInput(inputMsg([inputFloats]float64{math.NaN(), 7, math.Inf(1), -3, 999, 5}))
	if err != nil {
		t.Fatal(err)
	}
	want := sim.PlayerInput{MoveX: 0, MoveZ: 1, TargetX: 0, TargetZ: -3, Buttons: 255, Spin: 1}
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
	got, _ = decodeInput(inputMsg([inputFloats]float64{0, 0, 0, 0, -4, 0}))
	if got.Buttons != 0 {
		t.Fatalf("negative buttons -> %d", got.Buttons)
	}
}

func TestDecodeInputRejects(t *testing.T) {
	good := inputMsg([inputFloats]float64{})
	cases := map[string][]byte{
		"short":   good[:inputMsgLen-1],
		"long":    append(append([]byte{}, good...), 0),
		"type":    append([]byte{msgPing}, good[1:]...),
		"version": append([]byte{msgInput, protoVersion + 1}, good[2:]...),
	}
	for name, b := range cases {
		if _, err := decodeInput(b); err != errBadMessage {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestPingPong(t *testing.T) {
	ping := make([]byte, pingMsgLen)
	putHeader(ping, msgPing)
	putF64(ping, 0, 1234.5)
	if !isPing(ping) || isPing(ping[:pingMsgLen-1]) {
		t.Fatal("isPing")
	}
	bad := append([]byte{}, ping...)
	bad[1] = 9
	if isPing(bad) {
		t.Fatal("isPing accepted wrong version")
	}
	pong := encodePong(make([]byte, pingMsgLen), ping)
	if pong[0] != msgPong || pong[1] != protoVersion || f64(pong, 0) != 1234.5 {
		t.Fatalf("pong %v", pong)
	}
}

func TestEncodeWelcomeAndSignal(t *testing.T) {
	w := encodeWelcome(1)
	if len(w) != welcomeLen || w[0] != msgWelcome || w[1] != protoVersion || w[2] != 1 {
		t.Fatalf("welcome header %v", w[:headerLen])
	}
	if f64(w, 0) != sim.SnapLen || f64(w, 1) != game.OutLen {
		t.Fatalf("welcome body %v %v", f64(w, 0), f64(w, 1))
	}
	s := encodeSignal(msgPeerLeft)
	if len(s) != headerLen || s[0] != msgPeerLeft || s[1] != protoVersion {
		t.Fatalf("signal %v", s)
	}
}

func TestEncodeFrame(t *testing.T) {
	out := make([]float64, game.OutLen)
	for i := range out {
		out[i] = float64(i) * 0.5
	}
	dst := make([]byte, frameMsgLen)
	for i := range dst {
		dst[i] = 0xff
	}
	encodeFrame(dst, out)
	if dst[0] != msgFrame || dst[1] != protoVersion || dst[7] != 0 {
		t.Fatalf("frame header %v", dst[:headerLen])
	}
	for i, want := range out {
		got := math.Float64frombits(binary.LittleEndian.Uint64(dst[headerLen+i*8:]))
		if got != want {
			t.Fatalf("float %d = %v want %v", i, got, want)
		}
	}
}
