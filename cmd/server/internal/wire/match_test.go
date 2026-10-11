package wire

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/txny34/futenis/game"
	"github.com/txny34/futenis/sim"
)

func TestDecodeInput(t *testing.T) {
	got, err := DecodeInput(EncodeInput([InputFloats]float64{0.5, -0.25, 3, -2, 9, -1}))
	if err != nil {
		t.Fatal(err)
	}
	want := sim.PlayerInput{MoveX: 0.5, MoveZ: -0.25, TargetX: 3, TargetZ: -2, Buttons: 9, Spin: -1}
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestDecodeInputSanitizes(t *testing.T) {
	got, err := DecodeInput(EncodeInput([InputFloats]float64{math.NaN(), 7, math.Inf(1), -3, 999, 5}))
	if err != nil {
		t.Fatal(err)
	}
	want := sim.PlayerInput{MoveX: 0, MoveZ: 1, TargetX: 0, TargetZ: -3, Buttons: 255, Spin: 1}
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
	got, _ = DecodeInput(EncodeInput([InputFloats]float64{0, 0, 0, 0, -4, 0}))
	if got.Buttons != 0 {
		t.Fatalf("negative buttons -> %d", got.Buttons)
	}
}

func TestDecodeInputRejects(t *testing.T) {
	good := EncodeInput([InputFloats]float64{})
	cases := map[string][]byte{
		"short":   good[:InputLen-1],
		"long":    append(append([]byte{}, good...), 0),
		"type":    append([]byte{MsgPing}, good[1:]...),
		"version": append([]byte{MsgInput, Version + 1}, good[2:]...),
	}
	for name, b := range cases {
		if _, err := DecodeInput(b); err != ErrBadMessage {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestPingPong(t *testing.T) {
	ping := make([]byte, PingLen)
	PutHeader(ping, MsgPing)
	PutF64(ping, 0, 1234.5)
	if !IsPing(ping) || IsPing(ping[:PingLen-1]) {
		t.Fatal("isPing")
	}
	bad := append([]byte{}, ping...)
	bad[1] = 9
	if IsPing(bad) {
		t.Fatal("isPing accepted wrong version")
	}
	pong := EncodePong(make([]byte, PingLen), ping)
	if pong[0] != MsgPong || pong[1] != Version || F64(pong, 0) != 1234.5 {
		t.Fatalf("pong %v", pong)
	}
}

func TestEncodeWelcomeAndSignal(t *testing.T) {
	w := EncodeWelcome(1)
	if len(w) != WelcomeLen || w[0] != MsgWelcome || w[1] != Version || w[2] != 1 {
		t.Fatalf("welcome header %v", w[:HeaderLen])
	}
	if F64(w, 0) != sim.SnapLen || F64(w, 1) != game.OutLen {
		t.Fatalf("welcome body %v %v", F64(w, 0), F64(w, 1))
	}
	s := EncodeSignal(MsgPeerLeft)
	if len(s) != HeaderLen || s[0] != MsgPeerLeft || s[1] != Version {
		t.Fatalf("signal %v", s)
	}
}

func TestEncodeFrame(t *testing.T) {
	out := make([]float64, game.OutLen)
	for i := range out {
		out[i] = float64(i) * 0.5
	}
	dst := make([]byte, FrameLen)
	for i := range dst {
		dst[i] = 0xff
	}
	EncodeFrame(dst, out)
	if dst[0] != MsgFrame || dst[1] != Version || dst[7] != 0 {
		t.Fatalf("frame header %v", dst[:HeaderLen])
	}
	for i, want := range out {
		got := math.Float64frombits(binary.LittleEndian.Uint64(dst[HeaderLen+i*8:]))
		if got != want {
			t.Fatalf("float %d = %v want %v", i, got, want)
		}
	}
}
