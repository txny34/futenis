package main

import (
	"math"
	"testing"

	"github.com/txny34/futenis/ai"
	"github.com/txny34/futenis/sim"
)

func TestConfigureValidates(t *testing.T) {
	ss := defaultSession()
	bad := [][4]int{{3, 0, 5, 0}, {-1, 0, 5, 0}, {0, 4, 5, 0}, {0, 0, 0, 0}, {0, 0, 100, 0}, {0, 0, 5, 16}, {0, 0, 5, -1}}
	for _, b := range bad {
		if err := ss.configure(b[0], b[1], b[2], b[3]); err == nil {
			t.Errorf("configure%v: want error", b)
		}
	}
	if ss.cfg.Mode != sim.Mode1v1 {
		t.Fatal("failed configure must not change session")
	}
}

func TestConfigure2v2WithBots(t *testing.T) {
	ss := defaultSession()
	if err := ss.configure(int(sim.Mode2v2), int(ai.Hard), 7, 0b0001); err != nil {
		t.Fatal(err)
	}
	st := ss.match.State()
	if st.NumPlayers != 4 || st.Cfg.PointsToWin != 7 || ss.drv.humanMask != 1 {
		t.Fatalf("got players=%d points=%d mask=%d", st.NumPlayers, st.Cfg.PointsToWin, ss.drv.humanMask)
	}
	if ss.drv.bots[2].Difficulty() != ai.Hard {
		t.Fatal("bot difficulty")
	}
	ss.reset()
	if ss.drv.bots[3].Difficulty() != ai.Hard || ss.match.State().NumPlayers != 4 {
		t.Fatal("reset must keep configuration")
	}
}

func TestBotsDriveNonHumanSlots(t *testing.T) {
	ss := defaultSession()
	if err := ss.configure(int(sim.Mode1v1), int(ai.Pro), 5, 0); err != nil {
		t.Fatal(err)
	}
	out := make([]float64, outLen)
	start := ss.match.State().Players[0].Pos
	for i := 0; i < 40; i++ {
		runFrame(ss.match, [sim.MaxPlayers]sim.PlayerInput{}, 4, out, &ss.drv)
	}
	if ss.match.State().Players[0].Pos == start && ss.match.State().Tick == 0 {
		t.Fatal("match did not advance")
	}
	if out[outEventCount] < 0 {
		t.Fatal("event count")
	}
}

func TestHumanSlotsUntouchedByDriver(t *testing.T) {
	drv := newDriver(ai.Pro, 0b0011)
	m := sim.NewMatch(sim.DefaultConfig())
	in := [sim.MaxPlayers]sim.PlayerInput{{MoveX: 0.5}, {MoveZ: -0.5}}
	drv.fill(m, &in)
	if in[0].MoveX != 0.5 || in[1].MoveZ != -0.5 {
		t.Fatalf("human inputs overwritten: %+v", in)
	}
	var nilDrv *aiDriver
	nilDrv.fill(m, &in) // must not panic
}

func TestRunFrameWithBotsZeroAllocs(t *testing.T) {
	ss := defaultSession()
	_ = ss.configure(int(sim.Mode2v2), int(ai.Normal), 5, 0)
	out := make([]float64, outLen)
	allocs := testing.AllocsPerRun(200, func() {
		runFrame(ss.match, [sim.MaxPlayers]sim.PlayerInput{}, 4, out, &ss.drv)
	})
	if allocs != 0 {
		t.Fatalf("runFrame allocated %v", allocs)
	}
}

func TestRespawnBallPlaygroundOnly(t *testing.T) {
	ss := defaultSession()
	if ss.respawnBall() == nil {
		t.Fatal("1v1 respawn must fail")
	}
	_ = ss.configure(int(sim.ModePlayground), 0, 5, 0b11)
	if err := ss.respawnBall(); err != nil {
		t.Fatal(err)
	}
}

func TestConfigRoundTripKeepsModeAndPoints(t *testing.T) {
	ss := defaultSession()
	_ = ss.configure(int(sim.Mode2v2), 0, 9, 0)
	buf := make([]byte, cfgBytes)
	scratch := make([]float64, sim.CfgLen)
	ss.writeConfig(buf, scratch)
	scratch[0] = 5.0 // Gravity (first field in the layout)
	scratch[len(scratch)-1] = 0
	encodeFloats(buf, scratch)
	if err := ss.readConfig(buf, scratch); err != nil {
		t.Fatal(err)
	}
	st := ss.match.State()
	if st.Cfg.Gravity != 5 || st.Cfg.Mode != sim.Mode2v2 || st.Cfg.PointsToWin != 9 {
		t.Fatalf("cfg=%+v", st.Cfg)
	}
	encodeFloats(buf[:8], []float64{math.NaN()})
	if ss.readConfig(buf, scratch) == nil {
		t.Fatal("NaN must be rejected")
	}
}
