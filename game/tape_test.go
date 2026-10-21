package game

import (
	"bytes"
	"flag"
	"os"
	"slices"
	"testing"

	"github.com/txny34/futenis/ai"
	"github.com/txny34/futenis/sim"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata/tape_v2.bin")

// goldenTape is a fixed script also decoded by client/test/tape-format.test.ts.
func goldenTape() []byte {
	cfg := make([]byte, CfgBytes)
	for i := range cfg {
		cfg[i] = byte(i % 251)
	}
	w := NewTapeWriter(TapeSetup{Mode: int(sim.Mode2v2), Difficulty: int(ai.Hard), PointsToWin: 5, Mask: 0b0101, FirstServer: 1}, cfg, 1<<20)
	var in [sim.MaxPlayers]sim.PlayerInput
	w.Frame(&in, 2, 0b0101) // nothing changed vs zero: one record, changed=0
	w.Frame(&in, 2, 0b0101) // merged
	in[0] = sim.PlayerInput{MoveX: 0.5, MoveZ: -1, TargetX: 6.25, TargetZ: -2.5, Buttons: sim.BtnStrike, Spin: -1}
	w.Frame(&in, 3, 0b0101) // slot 0 changed
	in[2] = sim.PlayerInput{MoveX: -0.75, TargetX: 4, Buttons: 9, Spin: 1}
	in[1].MoveX = 1         // slot 1 is a bot: ignored
	w.Frame(&in, 2, 0b0101) // slot 2 changed
	w.Frame(&in, 2, 0b0001) // mask change: record, changed=0
	return w.Bytes()
}

func TestTapeGolden(t *testing.T) {
	got := goldenTape()
	path := "testdata/tape_v2.bin"
	if *updateGolden {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run: go test ./game -run TestTapeGolden -update)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("tape encoder output changed: tape v2 is frozen (client/src/replay/tape-format.ts reads it)")
	}
}

func TestTapeRoundTrip(t *testing.T) {
	s, cfg, steps, err := DecodeTape(goldenTape())
	if err != nil {
		t.Fatal(err)
	}
	if s.Mode != int(sim.Mode2v2) || s.Difficulty != int(ai.Hard) || s.PointsToWin != 5 || s.Mask != 0b0101 || s.FirstServer != 1 || cfg[7] != 7 {
		t.Fatalf("setup %+v cfg[7]=%d", s, cfg[7])
	}
	wantTicks := []int{4, 3, 2, 2}
	if len(steps) != len(wantTicks) {
		t.Fatalf("%d records, want %d", len(steps), len(wantTicks))
	}
	for i, w := range wantTicks {
		if steps[i].Ticks != w {
			t.Fatalf("record %d ticks %d want %d", i, steps[i].Ticks, w)
		}
	}
	if steps[1].In[0].TargetX != 6.25 || steps[1].In[0].Spin != -1 || steps[2].In[2].Buttons != 9 {
		t.Fatalf("inputs %+v", steps[2].In)
	}
	if steps[2].In[1].MoveX != 0 {
		t.Fatal("a bot slot's input was recorded")
	}
	if steps[3].Mask != 0b0001 || steps[3].In[2].Buttons != 9 {
		t.Fatalf("mask record %+v", steps[3])
	}
}

func TestTapeTooLongIsNil(t *testing.T) {
	w := NewTapeWriter(TapeSetup{Mask: 1}, make([]byte, CfgBytes), 10)
	var in [sim.MaxPlayers]sim.PlayerInput
	for i := 0; i < 8; i++ {
		w.Frame(&in, 2, 1)
	}
	if w.Bytes() != nil {
		t.Fatal("over maxTicks must give no tape")
	}
}

func TestTapeRejectsGarbage(t *testing.T) {
	if _, _, _, err := DecodeTape([]byte("nope")); err == nil {
		t.Fatal("garbage accepted")
	}
	b := goldenTape()
	if _, _, _, err := DecodeTape(b[:len(b)-3]); err == nil {
		t.Fatal("truncated tape accepted")
	}
}

// A tape replays a session exactly: record a scripted bot match, replay it.
// Only the current snapshot (outCurr..outEventCount) is compared: the
// replay's run splits differ from the recording's, so the "prev" snapshot
// slice legitimately diverges between the two while the current state and
// events at the point of comparison still must match exactly.
func TestTapeReplaysSession(t *testing.T) {
	g := NewSession()
	if err := g.Configure(int(sim.Mode1v1), int(ai.Normal), 5, 1); err != nil {
		t.Fatal(err)
	}
	cfg, scratch := make([]byte, CfgBytes), make([]float64, sim.CfgLen)
	g.WriteConfig(cfg, scratch)
	w := NewTapeWriter(TapeSetup{Mode: int(sim.Mode1v1), Difficulty: int(ai.Normal), PointsToWin: 5, Mask: 1}, cfg, 1<<20)
	out := make([]float64, OutLen)
	var in [sim.MaxPlayers]sim.PlayerInput
	for f := 0; f < 600; f++ {
		in[0] = sim.PlayerInput{MoveX: float64(f%7) / 7, TargetX: 6, TargetZ: float64(f%9) - 4}
		if f%40 < 10 {
			in[0].Buttons = sim.BtnStrike
		}
		g.Run(in, 2, out)
		w.Frame(&in, 2, 1)
	}
	want := append([]float64(nil), out...)
	s, c, steps, err := DecodeTape(w.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	r := NewSession()
	if err := r.Configure(s.Mode, s.Difficulty, s.PointsToWin, s.Mask); err != nil {
		t.Fatal(err)
	}
	if err := r.ReadConfig(c, scratch); err != nil {
		t.Fatal(err)
	}
	r.Reset()
	for _, st := range steps {
		for left := st.Ticks; left > 0; left -= MaxSteps {
			r.Run(st.In, min(left, MaxSteps), out)
		}
	}
	if !slices.Equal(out[outCurr:outEventCount], want[outCurr:outEventCount]) {
		t.Fatal("replayed session diverged")
	}
}

// What online replays rely on: a server-recorded 2v2 with first server 1 and
// a bot taking a seat mid-match replays identically from the tape alone.
func TestTapeReplaysFirstServerAndMaskChange(t *testing.T) {
	g := NewSession()
	if err := g.ConfigureFirst(int(sim.Mode2v2), int(ai.Normal), 5, 0b0101, 1); err != nil {
		t.Fatal(err)
	}
	cfg, scratch := make([]byte, CfgBytes), make([]float64, sim.CfgLen)
	g.WriteConfig(cfg, scratch)
	w := NewTapeWriter(TapeSetup{Mode: int(sim.Mode2v2), Difficulty: int(ai.Normal), PointsToWin: 5, Mask: 0b0101, FirstServer: g.FirstServer()}, cfg, 1<<20)
	out := make([]float64, OutLen)
	var in [sim.MaxPlayers]sim.PlayerInput
	mask := 0b0101
	for f := 0; f < 900; f++ {
		if f == 400 {
			mask = 0b0001 // slot 2 left: the bot plays it
			in[2] = sim.PlayerInput{}
			if err := g.SetHumanMask(mask); err != nil {
				t.Fatal(err)
			}
		}
		in[0] = sim.PlayerInput{MoveX: float64(f%5) / 5, TargetX: 6, TargetZ: float64(f%7) - 3}
		if mask&0b0100 != 0 {
			in[2] = sim.PlayerInput{MoveZ: float64(f%3) - 1, TargetX: -6}
		}
		if f%50 < 8 {
			in[0].Buttons = sim.BtnStrike
		}
		w.Frame(&in, 2, mask)
		g.Run(in, 2, out)
	}
	want := append([]float64(nil), out[outCurr:outEventCount]...)
	s, c, steps, err := DecodeTape(w.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	r := NewSession()
	if err := r.Configure(int(sim.Mode1v1), int(ai.Easy), 3, 1); err != nil { // a used session: parity must not matter
		t.Fatal(err)
	}
	if err := r.ConfigureFirst(s.Mode, s.Difficulty, s.PointsToWin, s.Mask, s.FirstServer); err != nil {
		t.Fatal(err)
	}
	if err := r.ReadConfig(c, scratch); err != nil {
		t.Fatal(err)
	}
	r.Reset()
	cur := s.Mask
	for _, st := range steps {
		if st.Mask != cur {
			cur = st.Mask
			if err := r.SetHumanMask(cur); err != nil {
				t.Fatal(err)
			}
		}
		for left := st.Ticks; left > 0; left -= MaxSteps {
			r.Run(st.In, min(left, MaxSteps), out)
		}
	}
	if !slices.Equal(want, out[outCurr:outEventCount]) {
		t.Fatal("replay with first server 1 and a mid-match bot diverged")
	}
}

func TestTapeTooBigIsNil(t *testing.T) {
	w := NewTapeWriter(TapeSetup{Mask: 1}, make([]byte, CfgBytes), 1<<30)
	var in [sim.MaxPlayers]sim.PlayerInput
	for f := 0; w.Bytes() != nil && f < TapeMaxBytes; f++ {
		in[0].TargetX = float64(f) // a new aim every frame
		w.Frame(&in, 1, 1)
	}
	if w.Bytes() != nil {
		t.Fatal("a tape over TapeMaxBytes must be dropped")
	}
}
