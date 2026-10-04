package sim

// Entrenamiento: the ball machine of ModePlayground (docs/design/
// training.md). The machine lives on side 1 and launches every ball with
// the real solver (solveLaunch), sampling its parameters deterministically
// with Hash64 over the tick and the feed counter -- never math/rand.

// FeedShots bits (Config.FeedShots, docs/design/training.md §1).
const (
	FeedShotDrive uint8 = 1 << iota // derecha: flat drive that always clears the net
	FeedShotLob                     // globo: ShotLob
	FeedShotSmash                   // remate: fast descending shot from feedSmashHeight
	FeedShotAll   = FeedShotDrive | FeedShotLob | FeedShotSmash
)

// Shot type indices stored in FeedState.Shot (bit position in FeedShots).
const (
	feedDrive uint8 = iota
	feedLob
	feedSmash
)

// Feed geometry/tuning that is not live-tunable (docs/design/training.md §2).
const (
	feedMouthHeight  = 1.0 // m, drive/lob mouth height
	feedSmashHeight  = 2.6 // m, smash mouth height
	feedDevMax       = 1.5 // m, target deviation radius at quality 0
	feedDriveSpeedLo = 10.0
	feedDriveSpeedHi = 22.0
	feedSmashSpeedLo = 18.0
	feedSmashSpeedHi = 28.0
	feedMinTargetX   = 0.5 // m, a deviated target never gets closer to the net

	machineZLimit, machineXLo, machineXHi = 4.7, 2.0, 9.7
	feedXLo, feedXHi                      = 1.0, 9.7
	minFeedInterval, maxFeedInterval      = 0.5, 5.0
	maxFeedValidBounce                    = 2
)

// FeedState is the ball machine's bookkeeping, appended at the end of State
// (docs/design/training.md §2). Zero value = no feed yet, nothing live.
type FeedState struct {
	Count    uint32 // feeds launched so far
	NextTick uint32 // tick of the next continuous feed; 0 = waiting for Match.Feed()
	Live     bool   // the current machine ball is still being judged
	Bounces  uint8  // floor bounces on side 0 of the current ball before the strike
	Struck   bool   // the human already touched the current ball
	Streak   uint16 // consecutive legal returns
	Best     uint16 // best Streak this session

	// Appended beyond the contract's list (needed by the snapshot / tests):
	MouthX, MouthZ float64 // position of the last machine mouth (SnapMachineX/Z)
	Shot           uint8   // feedDrive/feedLob/feedSmash of the last feed
}

// Feed launches a ball now if none is live (ModePlayground only). The launch itself happens inside the next Step so its
// EvFeed reaches the caller through Step's event slice like every other
// event, and the trigger is a plain State change (replayable).
func (m *Match) Feed() {
	s := &m.state
	if s.Cfg.Mode != ModePlayground || s.Feed.Live {
		return
	}
	s.Feed.NextTick = s.Tick + 1
}

// stepFeed launches the scheduled feed once its tick arrives.
func stepFeed(s *State, ev []Event) []Event {
	f := &s.Feed
	if f.Live || f.NextTick == 0 || s.Tick < f.NextTick {
		return ev
	}
	return launchFeed(s, ev)
}

// feedRand is a tiny splitmix64 stream seeded from the state.
type feedRand struct{ h uint64 }

func (r *feedRand) next() uint64 {
	r.h = splitmix64(r.h)
	return r.h
}

// unit returns a uniform float in [0, 1).
func (r *feedRand) unit() float64 { return float64(r.next()>>11) * 0x1p-53 }

// lerpIn returns a deterministic sample inside [lo, hi].
func (r *feedRand) lerpIn(lo, hi float64) float64 {
	return lo + float64((hi-lo)*r.unit())
}

// launchFeed places the ball at a sampled machine mouth and launches it
// toward a sampled target on side 0 with the real solver.
func launchFeed(s *State, ev []Event) []Event {
	cfg := &s.Cfg
	f := &s.Feed
	r := feedRand{h: Hash64(uint64(s.Tick)<<32 | uint64(f.Count))}

	f.MouthZ = r.lerpIn(cfg.MachineZMin, cfg.MachineZMax)
	f.MouthX = r.lerpIn(cfg.MachineXMin, cfg.MachineXMax)
	f.Shot = pickFeedShot(cfg.FeedShots, r.next())
	power := r.lerpIn(cfg.FeedPowerMin, cfg.FeedPowerMax)
	quality := r.lerpIn(cfg.FeedQualityMin, cfg.FeedQualityMax)
	tgt := feedTarget(cfg, &r, quality)

	from := Vec3{X: f.MouthX, Y: feedMouthHeight, Z: f.MouthZ}
	if f.Shot == feedSmash {
		from.Y = feedSmashHeight
	}
	s.Ball = Ball{Pos: from, Vel: feedVelocity(cfg, from, tgt, f.Shot, power)}

	f.Count++
	f.NextTick = 0
	f.Live, f.Struck, f.Bounces = true, false, 0
	return append(ev, Event{Kind: EvFeed, Tick: s.Tick, Pos: from, Side: 1, Player: -1, Vel: s.Ball.Vel})
}

// feedTarget samples the bounce target on side 0 and deviates it by up to
// (1-quality)*feedDevMax, keeping it inside the court and past the net.
func feedTarget(cfg *Config, r *feedRand, quality float64) Vec3 {
	tgt := Vec3{
		X: -r.lerpIn(cfg.FeedXMin, cfg.FeedXMax),
		Y: BallRadius,
		Z: r.lerpIn(cfg.FeedZMin, cfg.FeedZMax),
	}
	a, b := unitPair(r.next())
	off := clampLen(Vec3{X: a, Z: b}, 1).Scale(float64((1 - quality) * feedDevMax))
	limitZ := CourtHalfWidth - cfg.TargetWallMargin
	tgt.X = clamp(float64(tgt.X+off.X), -(CourtHalfLength - cfg.TargetWallMargin), -feedMinTargetX)
	tgt.Z = clamp(float64(tgt.Z+off.Z), -limitZ, limitZ)
	return tgt
}

// feedVelocity solves the launch for the sampled shot type: drive/smash go
// through driveTicks (ensureNetClearance: always over the net), the lob
// through lobTicks with power as the apex charge.
func feedVelocity(cfg *Config, from, tgt Vec3, shot uint8, power float64) Vec3 {
	switch shot {
	case feedLob:
		return solveLaunch(cfg, from, tgt, ShotLob, 0, power)
	case feedSmash:
		speed := feedSmashSpeedLo + float64((feedSmashSpeedHi-feedSmashSpeedLo)*power)
		return solveLaunch(cfg, from, tgt, ShotDrive, speed, 0)
	default:
		speed := feedDriveSpeedLo + float64((feedDriveSpeedHi-feedDriveSpeedLo)*power)
		return solveLaunch(cfg, from, tgt, ShotDrive, speed, 0)
	}
}

// pickFeedShot picks one of the enabled bits of mask (h mod enabled count),
// in bit order. An empty mask falls back to the drive.
func pickFeedShot(mask uint8, h uint64) uint8 {
	mask &= FeedShotAll
	if mask == 0 {
		return feedDrive
	}
	n := uint64(0)
	for b := uint8(0); b < 3; b++ {
		n += uint64(mask >> b & 1)
	}
	k := h % n
	for b := uint8(0); b < 3; b++ {
		if mask>>b&1 == 0 {
			continue
		}
		if k == 0 {
			return b
		}
		k--
	}
	return feedDrive
}

// sanitizeFeed bounds the Entrenamiento fields (docs/design/training.md §1).
func (c *Config) sanitizeFeed() {
	c.MachineZMin, c.MachineZMax = clampRange(c.MachineZMin, c.MachineZMax, -machineZLimit, machineZLimit)
	c.MachineXMin, c.MachineXMax = clampRange(c.MachineXMin, c.MachineXMax, machineXLo, machineXHi)
	c.FeedZMin, c.FeedZMax = clampRange(c.FeedZMin, c.FeedZMax, -machineZLimit, machineZLimit)
	c.FeedXMin, c.FeedXMax = clampRange(c.FeedXMin, c.FeedXMax, feedXLo, feedXHi)
	c.FeedPowerMin, c.FeedPowerMax = clampRange(c.FeedPowerMin, c.FeedPowerMax, 0, 1)
	c.FeedQualityMin, c.FeedQualityMax = clampRange(c.FeedQualityMin, c.FeedQualityMax, 0, 1)
	c.FeedInterval = clamp(c.FeedInterval, minFeedInterval, maxFeedInterval)
	c.FeedShots &= FeedShotAll
	if c.FeedShots == 0 {
		c.FeedShots = FeedShotDrive
	}
	if c.FeedValidBounce < 1 {
		c.FeedValidBounce = 1
	} else if c.FeedValidBounce > maxFeedValidBounce {
		c.FeedValidBounce = maxFeedValidBounce
	}
}

// clampRange clamps both ends of a [lo, hi] range into [min, max] and swaps
// them if lo > hi.
func clampRange(lo, hi, min, max float64) (float64, float64) {
	lo, hi = clamp(lo, min, max), clamp(hi, min, max)
	if lo > hi {
		return hi, lo
	}
	return lo, hi
}
