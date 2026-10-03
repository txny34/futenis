package sim

// Snapshot layout: flat []float64 the client reads as a Float64Array
// (little endian). Changing this layout requires updating
// client/src/snapshot.ts in the same commit.
//
// Contract v2 (docs/design/contact-v2.md §11.2) only ever APPENDS new
// fields after SnapLastTeam: every index at or before SnapLastTeam keeps
// its pre-contract-v2 meaning and value, including the one rename
// (SnapLastKicker -> SnapLastTeam; same index, now reads RallyState.LastTeam
// instead of the old RallyState.LastKicker).
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
	SnapLastTeam

	// --- Contract v2 additions, appended (docs/design/contact-v2.md §11.2,
	// "Modos y 2v2"): nothing above this line ever moves. ---

	SnapBallSpin
	SnapBallTossed
	SnapNumPlayers
	SnapMode

	SnapP0VelX
	SnapP0VelZ
	SnapP0Stamina
	SnapP0Charge
	SnapP0OnGround
	SnapP0Buffer
	SnapP0Flags

	SnapP1VelX
	SnapP1VelZ
	SnapP1Stamina
	SnapP1Charge
	SnapP1OnGround
	SnapP1Buffer
	SnapP1Flags

	// Players 2/3 (2v2 only; zeroed when State.NumPlayers < 4) need their
	// full position too -- P0/P1's X/Y/Z already exist above.
	SnapP2X
	SnapP2Y
	SnapP2Z
	SnapP2VelX
	SnapP2VelZ
	SnapP2Stamina
	SnapP2Charge
	SnapP2OnGround
	SnapP2Buffer
	SnapP2Flags

	SnapP3X
	SnapP3Y
	SnapP3Z
	SnapP3VelX
	SnapP3VelZ
	SnapP3Stamina
	SnapP3Charge
	SnapP3OnGround
	SnapP3Buffer
	SnapP3Flags

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
	dst[SnapLastTeam] = float64(s.Rally.LastTeam)

	dst[SnapBallSpin] = s.Ball.Spin
	dst[SnapBallTossed] = boolToFloat(s.Ball.Tossed)
	dst[SnapNumPlayers] = float64(s.NumPlayers)
	dst[SnapMode] = float64(s.Cfg.Mode)

	dst[SnapP2X], dst[SnapP2Y], dst[SnapP2Z] = s.Players[2].Pos.X, s.Players[2].Pos.Y, s.Players[2].Pos.Z
	dst[SnapP3X], dst[SnapP3Y], dst[SnapP3Z] = s.Players[3].Pos.X, s.Players[3].Pos.Y, s.Players[3].Pos.Z

	writePlayerExtras(s, dst)
}

// writePlayerExtras writes every player's Vel/Stamina/Charge/OnGround/
// Buffer/Flags block into dst, in SnapP0VelX../SnapP3Flags order.
func writePlayerExtras(s *State, dst []float64) {
	starts := [MaxPlayers]int{SnapP0VelX, SnapP1VelX, SnapP2VelX, SnapP3VelX}
	for i, base := range starts {
		p := &s.Players[i]
		dst[base+0] = p.Vel.X
		dst[base+1] = p.Vel.Z
		dst[base+2] = p.Stamina
		dst[base+3] = chargeFraction(p, &s.Cfg)
		dst[base+4] = boolToFloat(p.OnGround)
		dst[base+5] = float64(p.Buffer)
		dst[base+6] = playerFlags(p)
	}
}

// chargeFraction is Player.ChargeTicks normalized 0..1 by the Config's
// cached chargeMaxTicks (0 if Config was never Sanitized, to avoid a NaN).
func chargeFraction(p *Player, cfg *Config) float64 {
	if cfg.chargeMaxTicks <= 0 {
		return 0
	}
	return float64(p.ChargeTicks) / float64(cfg.chargeMaxTicks)
}

// playerFlags packs ShotType (bit 0) and "stamina locked" (bit 1) per
// docs/design/contact-v2.md §11.2: Flags = ShotType | (StaminaLock>0)<<1.
func playerFlags(p *Player) float64 {
	flags := uint8(p.ShotType)
	if p.StaminaLock > 0 {
		flags |= 1 << 1
	}
	return float64(flags)
}
