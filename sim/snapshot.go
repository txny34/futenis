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

	// Dash addition (docs/design/contact-v2.md "Dash"): appended at the end
	// so every index above keeps its position. One flag per player (1 =
	// DashTicks > 0, i.e. the dash impulse is currently overriding normal
	// accel) for client-side animation; the sim itself never reads these
	// back.
	SnapP0DashActive
	SnapP1DashActive
	SnapP2DashActive
	SnapP3DashActive

	// --- scoring additions (docs/design/contact-v2.md "Puntuación de
	// pádel"), appended after the Dash flags: nothing above this line ever
	// moves. Only meaningful when Cfg.ScoringMode == ScoringPadel; in
	// ScoringSimple, SnapPoints0/1 mirror the legacy flat Score and
	// SnapGames0/1/SnapSets0/1 stay 0. ---
	SnapPoints0 // pointDisplayCode(Score[0], Score[1], GoldenPoint): 0/15/30/40/Adv as 0..4
	SnapPoints1
	SnapGames0
	SnapGames1
	SnapSets0
	SnapSets1
	SnapServerPlayer // Rally.ServerPlayer: which State.Players index serves this point
	SnapRallyShots   // Rally.ShotCount: legal touches so far this rally

	// --- prediction additions (docs/design/contact-v2.md "Predicción
	// (marcador de pique y anillo de timing)"), appended after the scoring
	// block: nothing above this line ever moves. SnapLanding* describes the
	// BALL's next floor contact (PredictLanding), independent of any
	// player; SnapP*Contact* describes, per player, the earliest predicted
	// contact if that player stayed exactly where they are (PredictContact)
	// -- the client uses this to draw a landing marker and a per-player
	// "timing ring" that closes at the predicted contact tick. ---
	SnapLandingX
	SnapLandingZ
	SnapLandingTicks
	SnapLandingOk // 1 if PredictLanding found a floor contact within its horizon, else 0 (X/Z/Ticks are 0)

	SnapP0ContactTicks
	SnapP0ContactPart
	SnapP0ContactQuality
	SnapP0ContactOk

	SnapP1ContactTicks
	SnapP1ContactPart
	SnapP1ContactQuality
	SnapP1ContactOk

	SnapP2ContactTicks
	SnapP2ContactPart
	SnapP2ContactQuality
	SnapP2ContactOk

	SnapP3ContactTicks
	SnapP3ContactPart
	SnapP3ContactQuality
	SnapP3ContactOk

	// --- Entrenamiento additions (docs/design/training.md §3), appended
	// after the prediction block: nothing above this line ever moves. ---
	SnapStreak
	SnapBestStreak
	SnapFeedCount
	SnapFeedIn   // seconds until the next feed; -1 = waiting for demand or ball live
	SnapMachineX // last machine mouth position
	SnapMachineZ

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
	writeDashFlags(s, dst)
	writeScoring(s, dst)
	writePrediction(s, dst)
	writeFeed(s, dst)
}

// writeFeed fills the Entrenamiento block (docs/design/training.md §3).
func writeFeed(s *State, dst []float64) {
	f := &s.Feed
	dst[SnapStreak] = float64(f.Streak)
	dst[SnapBestStreak] = float64(f.Best)
	dst[SnapFeedCount] = float64(f.Count)
	dst[SnapFeedIn] = -1
	if !f.Live && f.NextTick != 0 {
		left := 0.0
		if f.NextTick > s.Tick {
			left = float64(f.NextTick - s.Tick)
		}
		dst[SnapFeedIn] = left / TickRate
	}
	dst[SnapMachineX] = f.MouthX
	dst[SnapMachineZ] = f.MouthZ
}

// writePrediction fills the "Predicción" snapshot block (docs/design/
// contact-v2.md "Predicción (marcador de pique y anillo de timing)"):
// PredictLanding once for the ball, PredictContact once per active player
// (inactive slots -- NumPlayers < 4 -- get SnapLen's zero value: Ok=0).
func writePrediction(s *State, dst []float64) {
	landing, landingTicks, landingOk := PredictLanding(s)
	dst[SnapLandingX] = landing.X
	dst[SnapLandingZ] = landing.Z
	dst[SnapLandingTicks] = float64(landingTicks)
	dst[SnapLandingOk] = boolToFloat(landingOk)

	starts := [MaxPlayers]int{SnapP0ContactTicks, SnapP1ContactTicks, SnapP2ContactTicks, SnapP3ContactTicks}
	for i, base := range starts {
		if int8(i) >= s.NumPlayers {
			continue
		}
		ticks, part, quality, ok := PredictContact(s, i)
		dst[base+0] = float64(ticks)
		dst[base+1] = float64(part)
		dst[base+2] = float64(quality)
		dst[base+3] = boolToFloat(ok)
	}
}

// writeScoring fills the "Puntuación de pádel" snapshot block
// (docs/design/contact-v2.md "Puntuación de pádel"). In ScoringSimple,
// Points0/1 mirror the flat Score (pointDisplayCode degenerates to the raw
// value for the low range Simple actually uses) and Games/Sets stay 0.
func writeScoring(s *State, dst []float64) {
	golden := s.Cfg.GoldenPoint
	dst[SnapPoints0] = float64(pointDisplayCode(s.Score[0], s.Score[1], golden))
	dst[SnapPoints1] = float64(pointDisplayCode(s.Score[1], s.Score[0], golden))
	dst[SnapGames0] = float64(s.Games[0])
	dst[SnapGames1] = float64(s.Games[1])
	dst[SnapSets0] = float64(s.Sets[0])
	dst[SnapSets1] = float64(s.Sets[1])
	dst[SnapServerPlayer] = float64(s.Rally.ServerPlayer)
	dst[SnapRallyShots] = float64(s.Rally.ShotCount)
}

// writeDashFlags fills the per-player DashActive flags appended at the end
// of the layout (docs/design/contact-v2.md "Dash").
func writeDashFlags(s *State, dst []float64) {
	flags := [MaxPlayers]int{SnapP0DashActive, SnapP1DashActive, SnapP2DashActive, SnapP3DashActive}
	for i, idx := range flags {
		dst[idx] = boolToFloat(s.Players[i].DashTicks > 0)
	}
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
