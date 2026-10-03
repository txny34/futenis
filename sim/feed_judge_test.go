package sim

import "testing"

// liveFeedState returns a Playground state with a live machine ball
// travelling toward the human (no Step needed: judgeFeed is pure on events).
func liveFeedState(t *testing.T, mutate func(c *Config)) State {
	t.Helper()
	cfg := DefaultConfig()
	cfg.Mode = ModePlayground
	if mutate != nil {
		mutate(&cfg)
		cfg.Sanitize()
	}
	s := NewMatch(cfg).State()
	s.Feed.Live = true
	s.Feed.Streak, s.Feed.Best = 3, 5
	return s
}

func ev(kind EventKind, side, player int8) Event {
	return Event{Kind: kind, Side: side, Player: player}
}

func countKind(evs []Event, k EventKind) int {
	n := 0
	for _, e := range evs {
		if e.Kind == k {
			n++
		}
	}
	return n
}

// judgeSeq feeds each event to judgeFeed as its own tick and returns every
// event judgeFeed appended (OK/Fail).
func judgeSeq(s *State, seq ...Event) []Event {
	var out []Event
	for _, e := range seq {
		buf := []Event{e}
		res := judgeFeed(s, buf)
		out = append(out, res[1:]...)
	}
	return out
}

func TestJudgeFeed_LegalReturnAfterOneBounce(t *testing.T) {
	s := liveFeedState(t, nil)
	out := judgeSeq(&s,
		ev(EvBounceFloor, 0, -1),
		ev(EvKick, 0, 0),
		ev(EvBounceFloor, 1, -1),
	)
	if countKind(out, EvReturnOK) != 1 || s.Feed.Streak != 4 || s.Feed.Best != 5 {
		t.Fatalf("out=%+v feed=%+v, want one OK and streak 4", out, s.Feed)
	}
	if out[0].Player != 0 {
		t.Errorf("EvReturnOK.Player: got %d, want 0", out[0].Player)
	}
	if s.Feed.Live || s.Feed.NextTick != s.Tick+uint32(s.Cfg.feedIntervalTicks) {
		t.Errorf("ball must die and schedule next feed: %+v", s.Feed)
	}
}

func TestJudgeFeed_BestTracksStreak(t *testing.T) {
	s := liveFeedState(t, nil)
	s.Feed.Streak, s.Feed.Best = 5, 5
	judgeSeq(&s, ev(EvKick, 0, 0), ev(EvBounceFloor, 1, -1))
	if s.Feed.Streak != 6 || s.Feed.Best != 6 {
		t.Fatalf("feed=%+v, want streak=best=6", s.Feed)
	}
}

func TestJudgeFeed_OnDemandLeavesNextTickZero(t *testing.T) {
	s := liveFeedState(t, func(c *Config) { c.FeedOnDemand = true })
	judgeSeq(&s, ev(EvKick, 0, 0), ev(EvBounceFloor, 1, -1))
	if s.Feed.Live || s.Feed.NextTick != 0 {
		t.Fatalf("feed=%+v, want dead ball waiting for demand", s.Feed)
	}
}

func TestJudgeFeed_Failures(t *testing.T) {
	volley := func(c *Config) { c.FeedVolleyOnly = true }
	cases := []struct {
		name   string
		mutate func(c *Config)
		seq    []Event
	}{
		{"double bounce", nil, []Event{ev(EvBounceFloor, 0, -1), ev(EvBounceFloor, 0, -1)}},
		{"volley-only bounce", volley, []Event{ev(EvBounceFloor, 0, -1)}},
		{"whiff", nil, []Event{ev(EvWhiff, 0, 0)}},
		{"out untouched", nil, []Event{ev(EvOut, 0, -1)}},
		{"rests untouched", func(c *Config) { c.FeedValidBounce = 2 }, []Event{ev(EvBounceFloor, 0, -1), {Kind: EvBounceFloor, Side: 0, Player: -2}}},
		{"return into net", nil, []Event{ev(EvKick, 0, 0), ev(EvHitNet, 0, -1)}},
		{"return out", nil, []Event{ev(EvKick, 0, 0), ev(EvOut, 1, -1)}},
		{"return bounces own side", nil, []Event{ev(EvKick, 0, 0), ev(EvBounceFloor, 0, -1)}},
		{"return hits rival glass first", nil, []Event{ev(EvKick, 0, 0), ev(EvHitWall, 1, -1)}},
		{"double touch", nil, []Event{ev(EvKick, 0, 0), ev(EvBodyHit, 0, 0)}},
		{"untouched comes back into net", nil, []Event{ev(EvBounceFloor, 0, -1), ev(EvHitNet, 0, -1)}},
		{"untouched comes back over", nil, []Event{ev(EvBounceFloor, 0, -1), ev(EvBounceFloor, 1, -1)}},
		{"whiff after return", nil, []Event{ev(EvKick, 0, 0), ev(EvWhiff, 0, 0)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := liveFeedState(t, c.mutate)
			seq := c.seq
			for i := range seq {
				if seq[i].Player == -2 { // marker: make the ball rest
					seq[i].Player = -1
					s.Ball.Resting = true
				}
			}
			out := judgeSeq(&s, seq...)
			if countKind(out, EvReturnFail) != 1 || countKind(out, EvReturnOK) != 0 {
				t.Fatalf("out=%+v, want exactly one EvReturnFail", out)
			}
			if s.Feed.Streak != 0 || s.Feed.Best != 5 || s.Feed.Live {
				t.Fatalf("feed=%+v, want streak reset, best kept, ball dead", s.Feed)
			}
		})
	}
}

func TestJudgeFeed_LegalVariants(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(c *Config)
		seq    []Event
	}{
		{"volley", func(c *Config) { c.FeedVolleyOnly = true }, []Event{ev(EvKick, 0, 0), ev(EvBounceFloor, 1, -1)}},
		{"two bounces allowed", func(c *Config) { c.FeedValidBounce = 2 }, []Event{
			ev(EvBounceFloor, 0, -1), ev(EvBounceFloor, 0, -1), ev(EvKick, 0, 0), ev(EvBounceFloor, 1, -1)}},
		{"own glass then over", nil, []Event{ev(EvKick, 0, 0), ev(EvHitWall, 0, -1), ev(EvBounceFloor, 1, -1)}},
		{"glass after bounce", nil, []Event{ev(EvBounceFloor, 0, -1), ev(EvHitWall, 0, -1), ev(EvKick, 0, 0), ev(EvBounceFloor, 1, -1)}},
		{"body hit counts", nil, []Event{ev(EvBodyHit, 0, 0), ev(EvBounceFloor, 1, -1)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := liveFeedState(t, c.mutate)
			out := judgeSeq(&s, c.seq...)
			if countKind(out, EvReturnOK) != 1 || countKind(out, EvReturnFail) != 0 || s.Feed.Streak != 4 {
				t.Fatalf("out=%+v feed=%+v, want one OK", out, s.Feed)
			}
		})
	}
}

func TestJudgeFeed_MachineFaultIsNeutral(t *testing.T) {
	for _, e := range []Event{ev(EvHitNet, 1, -1), ev(EvBounceFloor, 1, -1)} {
		s := liveFeedState(t, nil)
		out := judgeSeq(&s, e)
		if len(out) != 0 || s.Feed.Live || s.Feed.Streak != 3 {
			t.Fatalf("%+v: out=%+v feed=%+v, want silent dead ball keeping streak", e, out, s.Feed)
		}
	}
}

func TestJudgeFeed_DeadBallIgnoresEvents(t *testing.T) {
	s := liveFeedState(t, nil)
	s.Feed.Live = false
	out := judgeSeq(&s, ev(EvWhiff, 0, 0), ev(EvOut, 0, -1))
	if len(out) != 0 || s.Feed.Streak != 3 {
		t.Fatalf("dead ball must not be judged: out=%+v", out)
	}
}

func TestJudgeFeed_StreakSaturates(t *testing.T) {
	s := liveFeedState(t, nil)
	s.Feed.Streak, s.Feed.Best = 65535, 65535
	judgeSeq(&s, ev(EvKick, 0, 0), ev(EvBounceFloor, 1, -1))
	if s.Feed.Streak != 65535 || s.Feed.Best != 65535 {
		t.Fatalf("streak must saturate, got %+v", s.Feed)
	}
}

func TestApplyRules_NonPlaygroundIgnoresFeed(t *testing.T) {
	s := NewMatch(DefaultConfig()).State()
	s.Feed.Live = true
	res := applyRules(&s, []Event{ev(EvWhiff, 0, 0)})
	if countKind(res, EvReturnFail) != 0 {
		t.Fatal("1v1 must never judge feeds")
	}
}
