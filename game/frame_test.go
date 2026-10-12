package game

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
	src[4] = 1                // p0 buttons
	src[inputPerPlayer+4] = 0 // p1 buttons
	src[inputPerPlayer] = math.NaN()
	buf := make([]byte, inputLen*8)
	encodeFloats(buf, src)

	in := decodeInputs(buf)

	if in[0].MoveX != 0.5 || in[0].MoveZ != 1.5 || in[0].Buttons != 1 || in[0].TargetX != 2.5 {
		t.Fatalf("p0 decoded wrong: %+v", in[0])
	}
	if in[1].MoveX != 0 || in[1].Buttons != 0 || in[1].TargetX != 8.5 {
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
	writeEvent(out, 1, sim.Event{
		Kind: sim.EvKick, Tick: 7, Pos: sim.Vec3{X: 1, Y: 2, Z: 3}, Side: 1, Player: 2,
		Part: sim.PartFoot, Quality: sim.QualGood, Vel: sim.Vec3{X: 4, Y: 5, Z: 6}, Spin: 0.5,
	})
	want := []float64{
		float64(sim.EvKick), 7, 1, 2, 3, 1, 2,
		float64(sim.PartFoot), float64(sim.QualGood), 4, 5, 6, 0.5, 0,
	}
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
	if outLen != 2*sim.SnapLen+1+maxEvents*eventStride {
		t.Fatalf("outLen=%d", outLen)
	}
	if inputLen != sim.MaxPlayers*inputPerPlayer {
		t.Fatalf("inputLen=%d", inputLen)
	}
}

func TestRunFrameWritesCountAndCurr(t *testing.T) {
	m := sim.NewMatch(sim.DefaultConfig())
	out := make([]float64, outLen)
	for i := range out {
		out[i] = -999
	}
	runFrame(m, [sim.MaxPlayers]sim.PlayerInput{}, 3, out, nil)
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
	curr := out[outCurr:outEventCount]
	if curr[sim.SnapTick] != float64(st.Tick) || curr[sim.SnapNumPlayers] != 2 {
		t.Fatalf("tick=%v players=%v", curr[sim.SnapTick], curr[sim.SnapNumPlayers])
	}
	assertInactiveContactCleared(t, curr)
	assertBinary(t, curr, sim.SnapLandingOk, sim.SnapP0ContactOk, sim.SnapP1ContactOk)
}

// assertInactiveContactCleared: in 1v1, slots 2/3 have no PredictContact
// block; a reused (dirty) out buffer must still read Ok=0 there, never stale data.
func assertInactiveContactCleared(t *testing.T, curr []float64) {
	t.Helper()
	for _, base := range []int{sim.SnapP2ContactTicks, sim.SnapP3ContactTicks} {
		for k := 0; k < 4; k++ {
			if curr[base+k] != 0 {
				t.Fatalf("inactive contact slot curr[%d]=%v want 0", base+k, curr[base+k])
			}
		}
	}
}

func assertBinary(t *testing.T, curr []float64, idx ...int) {
	t.Helper()
	for _, i := range idx {
		if v := curr[i]; v != 0 && v != 1 {
			t.Fatalf("flag curr[%d]=%v want 0 or 1", i, v)
		}
	}
}

// TestRunFrameHeldInputPersistsAcrossTicks is a regression test for the v2
// contract change (docs/design/contact-v2.md §2): PlayerInput carries held
// levels, not edges, so runFrame must NOT clear anything between the ticks
// of a single step call -- a held BtnStrike across n>1 ticks must behave
// like it was held for real, not re-fire or vanish.
func TestRunFrameHeldInputPersistsAcrossTicks(t *testing.T) {
	m := sim.NewMatch(sim.DefaultConfig())
	in := [sim.MaxPlayers]sim.PlayerInput{{Buttons: sim.BtnStrike}}
	out := make([]float64, outLen)

	runFrame(m, in, 5, out, nil)

	st := m.State()
	if st.Players[0].Held&sim.BtnStrike == 0 {
		t.Fatalf("Player 0 Held: want BtnStrike still set after 5 ticks of a held input, got %+v", st.Players[0])
	}
}

func TestLayoutEntriesExposeLobbyKeys(t *testing.T) {
	want := map[string]float64{
		"snapLen": 94, "inputPerPlayer": 6, "eventStride": eventStride, "maxEvents": maxEvents, "cfgLen": 152,
		"modeLobby": 3, "snapLobbyPad": 91, "snapLobbyHold": 92,
		"evPadEnter": 24, "evPadLeave": 25, "evPadStart": 26, "evMatchPoint": 27,
		"cfgLobbyHoldTime": 72, "cfgLobbyHalfX": 73, "cfgLobbyHalfZ": 74, "lobbyPadRadius": 1.5,
	}
	got := layoutEntries()
	if len(got) != len(want) {
		t.Fatalf("got %d keys, want %d", len(got), len(want))
	}
	for _, e := range got {
		if w, ok := want[e.key]; !ok || w != e.val {
			t.Errorf("%s = %v, want %v (known=%v)", e.key, e.val, w, ok)
		}
	}
	pads := lobbyPadsFlat()
	wantPads := []float64{-6, -4, 0, -6, 6, -4}
	if len(pads) != len(wantPads) {
		t.Fatalf("lobbyPads=%v", pads)
	}
	for i, w := range wantPads {
		if pads[i] != w {
			t.Fatalf("lobbyPads[%d]=%v want %v", i, pads[i], w)
		}
	}
}
