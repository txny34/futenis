package game

import (
	"slices"
	"testing"

	"github.com/txny34/futenis/ai"
	"github.com/txny34/futenis/sim"
)

const (
	replayFrames = 900 // 7.5 s at 2 ticks per frame
	replayTicks  = 2
)

// playScripted runs a fixed human input script and returns the final frame.
func playScripted(g *Session) []float64 {
	out := make([]float64, OutLen)
	var in [sim.MaxPlayers]sim.PlayerInput
	for f := range replayFrames {
		in[0] = sim.PlayerInput{MoveX: float64(f%7) / 7, MoveZ: float64(f%5)/5 - 0.5, TargetX: 6, TargetZ: float64(f%9) - 4}
		if f%40 < 10 {
			in[0].Buttons = sim.BtnStrike
		}
		g.Run(in, replayTicks, out)
	}
	return out
}

// TestReplayNeedsStartConfig: the client replays a match by re-configuring the
// same session; configure alternates the first server, so the replay must
// restore the config snapshot taken at the original start (lobby/replay-flow.ts).
func TestReplayNeedsStartConfig(t *testing.T) {
	g := NewSession()
	if err := g.Configure(int(sim.Mode1v1), int(ai.Normal), 5, 1); err != nil {
		t.Fatal(err)
	}
	snap := make([]byte, CfgBytes)
	scratch := make([]float64, CfgBytes/8)
	g.WriteConfig(snap, scratch)
	g.Reset()
	original := playScripted(g)

	if err := g.Configure(int(sim.Mode1v1), int(ai.Normal), 5, 1); err != nil {
		t.Fatal(err)
	}
	g.Reset()
	if slices.Equal(original, playScripted(g)) {
		t.Fatal("re-configure alone replayed identically: the first-server flip is no longer covered")
	}

	if err := g.Configure(int(sim.Mode1v1), int(ai.Normal), 5, 1); err != nil {
		t.Fatal(err)
	}
	if err := g.ReadConfig(snap, scratch); err != nil {
		t.Fatal(err)
	}
	g.Reset()
	if !slices.Equal(original, playScripted(g)) {
		t.Fatal("replay with the start config diverged from the original")
	}
}
