package sim

// Simple scoring with sets (1v1/2v2, Config.SetsToWin > 1): each set is the
// flat "first to PointsToWin, win by 2, capped" tally; winning one bumps
// State.Sets, emits EvSetWon and starts the next set at 0-0; SetsToWin sets
// win the match (EvGameOver). With SetsToWin 1 the legacy single tally runs.

// playsSets reports whether s plays simple scoring in sets.
func playsSets(s *State) bool {
	return s.Cfg.SetsToWin > 1 && isMatchMode(s)
}

// isMatchMode reports a 1v1/2v2 match (the lobby and playground have none).
func isMatchMode(s *State) bool {
	return s.Cfg.Mode == Mode1v1 || s.Cfg.Mode == Mode2v2
}

// winSet closes the current set for winner: EvSetWon, then either the match
// (EvGameOver, PhaseGameOver, final score kept) or a fresh 0-0 set.
func winSet(s *State, winner int8, ev []Event) []Event {
	s.Sets[winner]++
	ev = append(ev, Event{Kind: EvSetWon, Tick: s.Tick, Side: winner, Player: -1})
	if s.Sets[winner] >= s.Cfg.SetsToWin {
		s.Phase = PhaseGameOver
		return append(ev, Event{Kind: EvGameOver, Tick: s.Tick, Side: winner, Player: -1})
	}
	s.Score = [2]int{}
	return ev
}

// appendMatchPoints warns, after a point, for each team one point away:
// EvMatchPoint when that point wins the match, EvSetPoint when it only wins
// the set (1v1/2v2 only).
func appendMatchPoints(s *State, ev []Event) []Event {
	if !isMatchMode(s) {
		return ev
	}
	for t := int8(0); t < 2; t++ {
		if !hasWonGame(s.Score[t]+1, s.Score[1-t], s.Cfg.PointsToWin) {
			continue
		}
		kind := EvMatchPoint
		if playsSets(s) && s.Sets[t]+1 < s.Cfg.SetsToWin {
			kind = EvSetPoint
		}
		ev = append(ev, Event{Kind: kind, Tick: s.Tick, Side: t, Player: -1})
	}
	return ev
}
