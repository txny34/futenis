package main

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/txny34/futenis/sim"
)

func TestDecodeInputsOrderAndSanitize(t *testing.T) {
	src := make([]float64, inputLen)
	for i := range src {
		src[i] = float64(i) + 0.5
	}
	src[2] = 1                // p0 kick
	src[inputPerPlayer+2] = 0 // p1 no kick
	src[inputPerPlayer] = math.NaN()
	buf := make([]byte, inputLen*8)
	encodeFloats(buf, src)

	in := decodeInputs(buf)

	if in[0].MoveX != 0.5 || in[0].MoveZ != 1.5 || !in[0].Kick || in[0].Power != 6.5 {
		t.Fatalf("p0 decoded wrong: %+v", in[0])
	}
	if in[1].MoveX != 0 || in[1].Kick || in[1].AimX != 10.5 || in[1].Power != 13.5 {
		t.Fatalf("p1 decoded wrong: %+v", in[1])
	}
}

func TestEncodeFloatsLittleEndian(t *testing.T) {
	buf := make([]byte, 16)
	encodeFloats(buf, []float64{1.25, -3})
	if got := math.Float64frombits(binary.LittleEndian.Uint64(buf[8:])); got != -3 {
		t.Fatalf("got %v", got)
	}
}

func TestWriteEventSlot(t *testing.T) {
	out := make([]float64, outLen)
	writeEvent(out, 1, sim.Event{Kind: sim.EvKick, Tick: 7, Pos: sim.Vec3{X: 1, Y: 2, Z: 3}, Side: 1})
	want := []float64{float64(sim.EvKick), 7, 1, 2, 3, 1}
	o := outEvents + eventStride
	for i, w := range want {
		if out[o+i] != w {
			t.Fatalf("field %d = %v, want %v", i, out[o+i], w)
		}
	}
}

func TestClamp(t *testing.T) {
	cases := []struct{ in, want int }{{-3, minSteps}, {0, minSteps}, {5, 5}, {99, maxSteps}}
	for _, c := range cases {
		if got := clamp(c.in, minSteps, maxSteps); got != c.want {
			t.Errorf("clamp(%d)=%d want %d", c.in, got, c.want)
		}
	}
}

func TestLayoutSize(t *testing.T) {
	if outLen != 2*sim.SnapLen+1+192 {
		t.Fatalf("outLen=%d", outLen)
	}
}

func TestRunFrameWritesCountAndCurr(t *testing.T) {
	m := sim.NewMatch(sim.DefaultConfig())
	out := make([]float64, outLen)
	for i := range out {
		out[i] = -999
	}
	runFrame(m, [2]sim.PlayerInput{}, 3, out)
	if c := out[outEventCount]; c < 0 || c > maxEvents {
		t.Fatalf("event count %v out of range", c)
	}
	st := m.State()
	want := make([]float64, sim.SnapLen)
	sim.WriteSnapshot(&st, want)
	for i, w := range want {
		if out[outCurr+i] != w {
			t.Fatalf("curr[%d]=%v want %v", i, out[outCurr+i], w)
		}
	}
}
