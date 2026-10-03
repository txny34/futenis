package sim

import (
	"math"
	"testing"
)

func playgroundCfg(mutate func(c *Config)) Config {
	cfg := DefaultConfig()
	cfg.Mode = ModePlayground
	if mutate != nil {
		mutate(&cfg)
	}
	cfg.Sanitize()
	return cfg
}

var noInput [MaxPlayers]PlayerInput

// stepUntil steps m until pred(events) is true or limit ticks pass.
func stepUntil(m *Match, limit int, pred func(ev []Event) bool) bool {
	for i := 0; i < limit; i++ {
		if pred(m.Step(noInput)) {
			return true
		}
	}
	return false
}

func hasKind(k EventKind) func([]Event) bool {
	return func(ev []Event) bool { return countKind(ev, k) > 0 }
}

func TestPlayground_SinglePlayerAndScheduledFeed(t *testing.T) {
	m := NewMatch(playgroundCfg(nil))
	s := m.State()
	if s.NumPlayers != 1 {
		t.Fatalf("NumPlayers: got %d, want 1", s.NumPlayers)
	}
	if s.Players[1] != (Player{}) {
		t.Fatalf("slot 1 must stay inactive/zero: %+v", s.Players[1])
	}
	if s.Feed.Live || s.Feed.NextTick != uint32(s.Cfg.feedIntervalTicks) {
		t.Fatalf("first feed must be scheduled one interval in: %+v", s.Feed)
	}
}

func TestPlayground_ContinuousFeedFiresAtNextTick(t *testing.T) {
	m := NewMatch(playgroundCfg(nil))
	want := m.State().Feed.NextTick
	for m.State().Tick+1 < want {
		if countKind(m.Step(noInput), EvFeed) != 0 {
			t.Fatalf("fed early at tick %d", m.State().Tick)
		}
	}
	evs := m.Step(noInput)
	s := m.State()
	if countKind(evs, EvFeed) != 1 || !s.Feed.Live || s.Feed.Count != 1 || s.Feed.NextTick != 0 {
		t.Fatalf("tick %d: events=%+v feed=%+v", s.Tick, evs, s.Feed)
	}
	var e Event
	for _, x := range evs {
		if x.Kind == EvFeed {
			e = x
		}
	}
	if e.Pos != s.Ball.Pos || e.Pos.X != s.Feed.MouthX || e.Pos.Z != s.Feed.MouthZ || e.Side != 1 {
		t.Fatalf("EvFeed must carry the mouth position: e=%+v ball=%+v", e, s.Ball.Pos)
	}
	if s.Ball.Vel.X >= 0 {
		t.Fatalf("ball must travel toward the human (X-): vel=%+v", s.Ball.Vel)
	}
}

func TestPlayground_OnDemandWaitsForFeed(t *testing.T) {
	m := NewMatch(playgroundCfg(func(c *Config) { c.FeedOnDemand = true }))
	if m.State().Feed.NextTick != 0 {
		t.Fatalf("on-demand must start waiting: %+v", m.State().Feed)
	}
	if stepUntil(m, 6*TickRate, hasKind(EvFeed)) {
		t.Fatal("on-demand fed without Match.Feed()")
	}
	m.Feed()
	if countKind(m.Step(noInput), EvFeed) != 1 {
		t.Fatal("Match.Feed must launch on the next Step")
	}
	m.Feed() // ball already live: no-op
	if m.State().Feed.NextTick != 0 || countKind(m.Step(noInput), EvFeed) != 0 {
		t.Fatal("Feed with a live ball must be ignored")
	}
}

func TestMatchFeed_IgnoredOutsidePlayground(t *testing.T) {
	m := NewMatch(DefaultConfig())
	m.Feed()
	if m.State().Feed != (FeedState{}) {
		t.Fatalf("Feed outside playground must be a no-op: %+v", m.State().Feed)
	}
}

func TestPlayground_UntouchedFeedFailsThenRefeeds(t *testing.T) {
	m := NewMatch(playgroundCfg(func(c *Config) { c.FeedInterval = 0.5 }))
	s := m.State()
	s.Players[0].Pos = Vec3{X: -9.6, Z: 4.6}
	m.Restore(s)
	if !stepUntil(m, 10*TickRate, hasKind(EvReturnFail)) {
		t.Fatal("an untouched feed must end in EvReturnFail")
	}
	if !stepUntil(m, 2*TickRate, hasKind(EvFeed)) {
		t.Fatal("continuous mode must feed again after the interval")
	}
	if m.State().Feed.Count != 2 {
		t.Fatalf("Count: got %d, want 2", m.State().Feed.Count)
	}
}

// TestFeed_QualityOneLandsInTargetZone: with quality 1 every drive/smash
// clears the net and first bounces inside the configured target zone.
func TestFeed_QualityOneLandsInTargetZone(t *testing.T) {
	for _, shot := range []uint8{FeedShotDrive, FeedShotSmash, FeedShotLob} {
		cfg := playgroundCfg(func(c *Config) {
			c.FeedShots = shot
			c.FeedQualityMin, c.FeedQualityMax = 1, 1
			c.FeedPowerMin, c.FeedPowerMax = 0, 1
			c.FeedZMin, c.FeedZMax = -3, 3
			c.FeedXMin, c.FeedXMax = 2, 8.5
		})
		for i := 0; i < 60; i++ {
			s := NewMatch(cfg).State()
			s.Tick = uint32(i * 7919)
			s.Feed.Count = uint32(i)
			s.Players[0].Pos = Vec3{X: -9.6, Z: 4.6}
			launchFeed(&s, nil)
			checkFeedFlight(t, shot, s)
		}
	}
}

func checkFeedFlight(t *testing.T, shot uint8, s State) {
	t.Helper()
	m := &Match{state: s, events: make([]Event, 0, 32)}
	const slack = 0.35 // one tick of travel past the exact solver landing
	for i := 0; i < 6*TickRate; i++ {
		for _, e := range m.Step(noInput) {
			switch e.Kind {
			case EvHitNet:
				t.Fatalf("shot %d hit the net: %+v", shot, s.Feed)
			case EvHitWall, EvOut:
				t.Fatalf("shot %d hit a wall before bouncing: %+v", shot, e)
			case EvBounceFloor:
				x, z := -e.Pos.X, e.Pos.Z
				if x < s.Cfg.FeedXMin-slack || x > s.Cfg.FeedXMax+slack || math.Abs(z) > 3+slack {
					t.Fatalf("shot %d first bounce outside zone: %+v", shot, e.Pos)
				}
				return
			}
		}
	}
	t.Fatalf("shot %d never bounced", shot)
}

func TestFeed_SmashDescends(t *testing.T) {
	cfg := playgroundCfg(func(c *Config) { c.FeedShots = FeedShotSmash; c.FeedPowerMin = 1; c.FeedPowerMax = 1 })
	s := NewMatch(cfg).State()
	launchFeed(&s, nil)
	if s.Ball.Pos.Y != feedSmashHeight || s.Ball.Vel.Y >= 0 {
		t.Fatalf("smash must leave from %v going down: pos=%+v vel=%+v", feedSmashHeight, s.Ball.Pos, s.Ball.Vel)
	}
}

func TestFeed_SamplingIsDeterministicAndInRange(t *testing.T) {
	cfg := playgroundCfg(func(c *Config) { c.FeedShots = 7 })
	seen := [3]bool{}
	for i := 0; i < 200; i++ {
		a := NewMatch(cfg).State()
		a.Tick, a.Feed.Count = uint32(i*31), uint32(i)
		b := a
		launchFeed(&a, nil)
		launchFeed(&b, nil)
		if a != b {
			t.Fatal("same tick+count must give the same launch")
		}
		if a.Feed.MouthZ < cfg.MachineZMin || a.Feed.MouthZ > cfg.MachineZMax ||
			a.Feed.MouthX < cfg.MachineXMin || a.Feed.MouthX > cfg.MachineXMax {
			t.Fatalf("mouth out of range: %+v", a.Feed)
		}
		seen[a.Feed.Shot] = true
	}
	if seen != [3]bool{true, true, true} {
		t.Fatalf("every enabled shot type must be sampled: %v", seen)
	}
}

func TestPickFeedShot(t *testing.T) {
	cases := []struct {
		mask uint8
		h    uint64
		want uint8
	}{
		{FeedShotDrive, 5, feedDrive}, {FeedShotLob, 9, feedLob}, {FeedShotSmash, 2, feedSmash},
		{FeedShotLob | FeedShotSmash, 0, feedLob}, {FeedShotLob | FeedShotSmash, 1, feedSmash},
		{0, 3, feedDrive},
	}
	for _, c := range cases {
		if got := pickFeedShot(c.mask, c.h); got != c.want {
			t.Errorf("pickFeedShot(%b,%d): got %d, want %d", c.mask, c.h, got, c.want)
		}
	}
}

func TestSnapshot_FeedFields(t *testing.T) {
	m := NewMatch(playgroundCfg(nil))
	dst := make([]float64, SnapLen)
	s := m.State()
	s.Feed.Streak, s.Feed.Best, s.Feed.Count = 4, 9, 12
	s.Feed.MouthX, s.Feed.MouthZ = 8.5, -1.25
	s.Tick, s.Feed.NextTick = 100, 160
	WriteSnapshot(&s, dst)
	if dst[SnapStreak] != 4 || dst[SnapBestStreak] != 9 || dst[SnapFeedCount] != 12 ||
		dst[SnapMachineX] != 8.5 || dst[SnapMachineZ] != -1.25 {
		t.Fatalf("feed snapshot fields wrong: %v", dst[SnapStreak:SnapLen])
	}
	assertFloat(t, "SnapFeedIn", dst[SnapFeedIn], 60.0/TickRate)
	s.Feed.Live = true
	WriteSnapshot(&s, dst)
	if dst[SnapFeedIn] != -1 {
		t.Fatalf("live ball: SnapFeedIn got %v, want -1", dst[SnapFeedIn])
	}
	s.Feed.Live, s.Feed.NextTick = false, 0
	WriteSnapshot(&s, dst)
	if dst[SnapFeedIn] != -1 {
		t.Fatalf("waiting demand: SnapFeedIn got %v, want -1", dst[SnapFeedIn])
	}
	s.Feed.NextTick = 50 // overdue (e.g. just restored): clamp at 0
	WriteSnapshot(&s, dst)
	if dst[SnapFeedIn] != 0 {
		t.Fatalf("overdue: SnapFeedIn got %v, want 0", dst[SnapFeedIn])
	}
	if SnapStreak != SnapP3ContactOk+1 || SnapMachineZ != SnapLen-1 {
		t.Fatal("feed snapshot fields must be appended at the end")
	}
}
