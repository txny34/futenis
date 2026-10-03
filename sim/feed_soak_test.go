package sim

import "testing"

// feedStriker is a scripted, deterministic "auto striker" for the ball
// machine: it holds Strike while a feed is incoming and, once the ball is at
// foot height on its own half, teleports the human next to it and releases
// the shot toward a rival-half target. Every few feeds it deliberately does
// nothing (untouched ball) or swings at air (whiff) so both outcomes of the
// judge get exercised.
type feedStriker struct {
	g        *lcg
	released bool
	lastFeed uint32
	plan     uint64
}

func (fs *feedStriker) input(m *Match) [MaxPlayers]PlayerInput {
	s := m.State()
	var in [MaxPlayers]PlayerInput
	if !s.Feed.Live || s.Feed.Struck {
		return in
	}
	if s.Feed.Count != fs.lastFeed {
		fs.lastFeed, fs.released = s.Feed.Count, false
		fs.plan = fs.g.next() % 8
	}
	if fs.plan == 0 || fs.released { // plan 0: let it go untouched
		return in
	}
	in[0] = PlayerInput{TargetX: fs.g.floatRange(3, 8), TargetZ: fs.g.floatRange(-3, 3), Buttons: BtnStrike}
	if fs.plan == 1 && s.Ball.Pos.X < -2 { // plan 1: whiff (release far from the ball)
		in[0].Buttons, fs.released = 0, true
		return in
	}
	b := s.Ball
	if b.Pos.X < -1 && b.Pos.Y < 0.45 && b.Pos.Y > 0.12 {
		s.Players[0].Pos = Vec3{X: b.Pos.X - 0.35, Z: b.Pos.Z}
		s.Players[0].Vel = Vec3{}
		m.Restore(s)
		in[0].Buttons, fs.released = 0, true
	}
	return in
}

type feedTally struct {
	ok, fail, feeds int
	streak, best    uint16
}

func runFeedSoak(t *testing.T, feeds int) (State, feedTally) {
	t.Helper()
	m := NewMatch(playgroundCfg(func(c *Config) { c.FeedInterval = 0.5; c.FeedShots = 7 }))
	fs := &feedStriker{g: newLCG(42)}
	var tl feedTally
	for tick := 0; tl.feeds < feeds && tick < feeds*12*TickRate; tick++ {
		for _, e := range m.Step(fs.input(m)) {
			switch e.Kind {
			case EvFeed:
				tl.feeds++
			case EvReturnOK:
				tl.ok++
				tl.streak++
				tl.best = max(tl.best, tl.streak)
			case EvReturnFail:
				tl.fail++
				tl.streak = 0
			}
		}
		s := m.State()
		assertFinite(t, tick, s)
		if s.Feed.Streak != tl.streak || s.Feed.Best != tl.best {
			t.Fatalf("tick %d: feed=%+v, tally streak=%d best=%d", tick, s.Feed, tl.streak, tl.best)
		}
	}
	return m.State(), tl
}

func TestSoak_FeedContinuous(t *testing.T) {
	feeds := 400
	if testing.Short() {
		feeds = 120
	}
	a, ta := runFeedSoak(t, feeds)
	b, tb := runFeedSoak(t, feeds)
	if a != b || ta != tb {
		t.Fatal("same inputs must give the same state (determinism)")
	}
	if ta.feeds < feeds {
		t.Fatalf("only %d feeds happened, want %d", ta.feeds, feeds)
	}
	if ta.ok == 0 || ta.fail == 0 {
		t.Fatalf("want both outcomes exercised: %+v", ta)
	}
	if ta.ok+ta.fail < feeds-1 { // only the last feed may still be live
		t.Fatalf("every feed must be judged: %+v", ta)
	}
	t.Logf("feeds=%d ok=%d fail=%d best=%d", ta.feeds, ta.ok, ta.fail, ta.best)
}

func TestFeedStep_ZeroAllocs(t *testing.T) {
	m := NewMatch(playgroundCfg(func(c *Config) { c.FeedInterval = 0.5; c.FeedShots = 7 }))
	fs := &feedStriker{g: newLCG(7)}
	dst := make([]float64, SnapLen)
	allocs := testing.AllocsPerRun(5000, func() {
		m.Step(fs.input(m))
		s := m.State()
		WriteSnapshot(&s, dst)
	})
	if allocs != 0 {
		t.Fatalf("Step+snapshot in playground allocated %v per tick", allocs)
	}
	if m.State().Feed.Count == 0 {
		t.Fatal("the alloc run never fed a ball")
	}
}
