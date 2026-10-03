package sim

// Snapshot layout: flat []float64 the client reads as a Float64Array (little endian).
// Changing this layout requires updating client/src/snapshot.ts in the same commit.
const (
	SnapTick = iota
	SnapPhase
	SnapServer
	SnapScore0
	SnapScore1
	SnapBallX
	SnapBallY
	SnapBallZ
	SnapP0X
	SnapP0Y
	SnapP0Z
	SnapP1X
	SnapP1Y
	SnapP1Z
	SnapLastKicker
	SnapLen
)

// WriteSnapshot fills dst (len >= SnapLen) without allocating.
func WriteSnapshot(s *State, dst []float64) {
	dst[SnapTick] = float64(s.Tick)
	dst[SnapPhase] = float64(s.Phase)
	dst[SnapServer] = float64(s.Server)
	dst[SnapScore0] = float64(s.Score[0])
	dst[SnapScore1] = float64(s.Score[1])
	dst[SnapBallX], dst[SnapBallY], dst[SnapBallZ] = s.Ball.Pos.X, s.Ball.Pos.Y, s.Ball.Pos.Z
	p0, p1 := s.Players[0].Pos, s.Players[1].Pos
	dst[SnapP0X], dst[SnapP0Y], dst[SnapP0Z] = p0.X, p0.Y, p0.Z
	dst[SnapP1X], dst[SnapP1Y], dst[SnapP1Z] = p1.X, p1.Y, p1.Z
	dst[SnapLastKicker] = float64(s.Rally.LastKicker)
}
