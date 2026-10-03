package sim

import "math"

// feedVerdict is the outcome of judging one event against the live feed.
type feedVerdict uint8

const (
	feedPending feedVerdict = iota // keep judging
	feedOK                         // legal return: Streak++
	feedFail                       // streak broken
	feedVoid                       // machine fault (e.g. its ball hit the net): dies silently
)

// applyPlayground is applyRules for ModePlayground (docs/design/training.md
// §2): no padel scoring, only the ball-machine judge and the feed clock.
func applyPlayground(s *State, ev []Event) []Event {
	ev = judgeFeed(s, ev)
	return stepFeed(s, ev)
}

// judgeFeed walks this tick's events (in emission order) while a feed is
// live and resolves it into EvReturnOK / EvReturnFail. Events it appends
// itself are never re-judged.
func judgeFeed(s *State, ev []Event) []Event {
	n := len(ev)
	for i := 0; i < n && s.Feed.Live; i++ {
		v := judgeFeedEvent(s, ev[i])
		if v == feedPending {
			continue
		}
		ev = resolveFeed(s, v, ev)
	}
	return ev
}

// judgeFeedEvent applies the OK/fail rules to a single event.
func judgeFeedEvent(s *State, e Event) feedVerdict {
	if e.Kind == EvWhiff {
		return feedFail
	}
	if s.Feed.Struck {
		return judgeReturnFlight(e)
	}
	return judgeIncoming(s, e)
}

// judgeIncoming handles the machine ball before the human touched it.
func judgeIncoming(s *State, e Event) feedVerdict {
	f := &s.Feed
	switch e.Kind {
	case EvKick, EvBodyHit:
		// FeedVolleyOnly needs no check here: its first bounce already failed.
		f.Struck = true
		return feedPending
	case EvBounceFloor:
		if e.Side != 0 {
			return feedOrVoid(f.Bounces > 0) // came back over untouched, or the machine fed short
		}
		f.Bounces++
		if s.Cfg.FeedVolleyOnly || f.Bounces > s.Cfg.FeedValidBounce || s.Ball.Resting {
			return feedFail
		}
		return feedPending
	case EvOut:
		return feedFail
	case EvHitNet:
		return feedOrVoid(e.Side == 0) // from side 0: it got past the human and came back
	default:
		return feedPending
	}
}

// feedOrVoid returns feedFail when the human is to blame, else feedVoid.
func feedOrVoid(humanFault bool) feedVerdict {
	if humanFault {
		return feedFail
	}
	return feedVoid
}

// judgeReturnFlight handles the human's return until its first bounce.
func judgeReturnFlight(e Event) feedVerdict {
	switch e.Kind {
	case EvBounceFloor:
		if e.Side == 1 {
			return feedOK
		}
		return feedFail
	case EvKick, EvBodyHit, EvHitNet, EvOut:
		return feedFail
	case EvHitWall:
		if e.Side == 1 {
			return feedFail
		}
		return feedPending
	default:
		return feedPending
	}
}

// resolveFeed updates the streak, emits the verdict event and kills the
// ball, scheduling the next continuous feed.
func resolveFeed(s *State, v feedVerdict, ev []Event) []Event {
	f := &s.Feed
	switch v {
	case feedOK:
		if f.Streak < math.MaxUint16 {
			f.Streak++
		}
		f.Best = max(f.Best, f.Streak)
		ev = append(ev, Event{Kind: EvReturnOK, Tick: s.Tick, Pos: s.Ball.Pos, Side: 0, Player: 0})
	case feedFail:
		f.Streak = 0
		ev = append(ev, Event{Kind: EvReturnFail, Tick: s.Tick, Pos: s.Ball.Pos, Side: 0, Player: -1})
	}
	f.Live, f.Struck, f.Bounces = false, false, 0
	f.NextTick = 0
	if !s.Cfg.FeedOnDemand {
		f.NextTick = s.Tick + uint32(s.Cfg.feedIntervalTicks)
	}
	return ev
}
