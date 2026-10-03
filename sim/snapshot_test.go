package sim

import (
	"strconv"
	"testing"
)

func TestWriteSnapshot(t *testing.T) {
	s := State{
		Tick:   123,
		Phase:  PhaseRally,
		Server: 1,
		Score:  [2]int{4, 7},
		Ball:   Ball{Pos: Vec3{X: 1, Y: 2, Z: 3}},
		Players: [2]Player{
			{Pos: Vec3{X: -5, Y: 0, Z: 1}},
			{Pos: Vec3{X: 5, Y: 0, Z: -1}},
		},
		Rally: RallyState{LastKicker: 1},
	}

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
		SnapLastKicker: 1,
	}

	for idx, w := range want {
		assertFloat(t, "dst["+strconv.Itoa(idx)+"]", dst[idx], w)
	}
}

func TestWriteSnapshotDoesNotAllocate(t *testing.T) {
	s := State{Ball: Ball{Pos: Vec3{X: 1, Y: 2, Z: 3}}}
	dst := make([]float64, SnapLen)

	allocs := testing.AllocsPerRun(100, func() {
		WriteSnapshot(&s, dst)
	})

	if allocs != 0 {
		t.Errorf("WriteSnapshot allocated %v times per call, want 0", allocs)
	}
}
