package sim

import (
	"strconv"
	"testing"
)

func TestWriteSnapshot(t *testing.T) {
	s := State{
		Tick:       123,
		Phase:      PhaseRally,
		Server:     1,
		NumPlayers: 2,
		Score:      [2]int{4, 7},
		Ball:       Ball{Pos: Vec3{X: 1, Y: 2, Z: 3}, Spin: 0.4, Tossed: true},
		Players: [MaxPlayers]Player{
			{Pos: Vec3{X: -5, Y: 0, Z: 1}, Vel: Vec3{X: 0.5, Z: 0.25}, Stamina: 0.8, Buffer: 3},
			{Pos: Vec3{X: 5, Y: 0, Z: -1}, Vel: Vec3{X: -0.5, Z: -0.25}, Stamina: 0.6, Buffer: 1},
			{},
			{},
		},
		Rally: RallyState{LastTeam: 1},
		Cfg:   DefaultConfig(),
	}
	s.Cfg.Mode = Mode2v2

	dst := make([]float64, SnapLen)
	WriteSnapshot(&s, dst)

	want := map[int]float64{
		SnapTick:       123,
		SnapPhase:      float64(PhaseRally),
		SnapServer:     1,
		SnapScore0:     4,
		SnapScore1:     7,
		SnapBallX:      1,
		SnapBallY:      2,
		SnapBallZ:      3,
		SnapP0X:        -5,
		SnapP0Y:        0,
		SnapP0Z:        1,
		SnapP1X:        5,
		SnapP1Y:        0,
		SnapP1Z:        -1,
		SnapLastTeam:   1,
		SnapBallSpin:   0.4,
		SnapBallTossed: 1,
		SnapNumPlayers: 2,
		SnapMode:       float64(Mode2v2),
		SnapP0VelX:     0.5,
		SnapP0VelZ:     0.25,
		SnapP0Stamina:  0.8,
		SnapP0Buffer:   3,
		SnapP1VelX:     -0.5,
		SnapP1VelZ:     -0.25,
		SnapP1Stamina:  0.6,
		SnapP1Buffer:   1,
		SnapP2X:        0,
		SnapP3X:        0,
	}

	for idx, w := range want {
		assertFloat(t, "dst["+strconv.Itoa(idx)+"]", dst[idx], w)
	}
}

func TestWriteSnapshotChargeFraction(t *testing.T) {
	s := State{Cfg: DefaultConfig()}
	s.Players[0].ChargeTicks = uint16(s.Cfg.chargeMaxTicks / 2)

	dst := make([]float64, SnapLen)
	WriteSnapshot(&s, dst)

	assertFloat(t, "dst[SnapP0Charge]", dst[SnapP0Charge], 0.5)
}

func TestWriteSnapshotFlags(t *testing.T) {
	s := State{Cfg: DefaultConfig()}
	s.Players[0].ShotType = ShotLob
	s.Players[0].StaminaLock = 5

	dst := make([]float64, SnapLen)
	WriteSnapshot(&s, dst)

	want := float64(uint8(ShotLob) | 1<<1)
	assertFloat(t, "dst[SnapP0Flags]", dst[SnapP0Flags], want)
}

func TestWriteSnapshotDoesNotAllocate(t *testing.T) {
	s := State{Cfg: DefaultConfig(), Ball: Ball{Pos: Vec3{X: 1, Y: 2, Z: 3}}}
	dst := make([]float64, SnapLen)

	allocs := testing.AllocsPerRun(100, func() {
		WriteSnapshot(&s, dst)
	})

	if allocs != 0 {
		t.Errorf("WriteSnapshot allocated %v times per call, want 0", allocs)
	}
}
