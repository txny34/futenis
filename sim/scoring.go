package sim

// Real-padel points/games/sets scoring (docs/design/contact-v2.md
// "Puntuación de pádel"). ScoringSimple (Config's zero value) keeps the
// legacy "first to PointsToWin, win by 2, deuceCap" behaviour untouched
// (rules.go's awardPointSimple/hasWonGame) so every pre-existing test and
// Playground/tuning use keeps working without opting in. ScoringPadel is
// the real 0/15/30/40/Adv -> games -> sets model implemented below.

// ScoringMode selects how awardPoint resolves a decided point into the
// match's score (docs/design/contact-v2.md "Puntuación de pádel").
type ScoringMode uint8

const (
	// ScoringSimple is the legacy behaviour: State.Score is a flat point
	// tally, first to Config.PointsToWin wins by 2 (hasWonGame/deuceCap).
	// The zero value, so every Config built without setting ScoringMode
	// (DefaultConfig, every existing test) is unaffected.
	ScoringSimple ScoringMode = iota
	// ScoringPadel is the real padel model: points (0/15/30/40/Adv) ->
	// games (Config.GamesPerSet, win by 2, tiebreak at GamesPerSet-all) ->
	// sets (Config.SetsToWin).
	ScoringPadel
)

// tiebreakTarget is how many points win a tiebreak game (win by 2), fixed
// at the real-padel/tennis value regardless of Config.GamesPerSet.
const tiebreakTarget = 7

// hasWonGamePadel reports whether pointsW (the team that just scored) has
// won the current game against pointsL: either the classic "first to
// target (4, or tiebreakTarget in a tiebreak), win by 2" rule, or, with
// Config.GoldenPoint, sudden death the instant either team leads at 40-40
// (3-3) or later, no 2-point margin required ("punto de oro").
func hasWonGamePadel(pointsW, pointsL, target int, golden bool) bool {
	if golden && pointsL >= target-1 && pointsW > pointsL {
		return true
	}
	return pointsW >= target && pointsW-pointsL >= 2
}

// gamePoints is the point target of a normal (non-tiebreak) game.
const gamePoints = 4

// hasWonSetGames reports whether gamesW (the team that just won a game) has
// won the set outright: first to target games, ahead by 2. Does not itself
// decide the tiebreak case (setNeedsTiebreak); a tiebreak's winner is
// awarded the set directly by awardGamePadel.
func hasWonSetGames(gamesW, gamesL, target int) bool {
	return gamesW >= target && gamesW-gamesL >= 2
}

// setNeedsTiebreak reports whether both teams are tied at exactly target
// games, the real-padel/tennis trigger for a single tiebreak game instead
// of playing on indefinitely.
func setNeedsTiebreak(g0, g1, target int) bool {
	return g0 == target && g1 == target
}

// pointDisplayCode maps a team's raw in-game point count (mine) against the
// opponent's (other) to the real-padel display value: 0=0, 1=15, 2=30,
// 3=40, 4=Adv. Golden-point matches never reach Adv (the game ends the
// instant either team leads past 3-3), so code 4 only appears without
// GoldenPoint. A count pair that has already clinched the game (hasWon in
// either direction -- a lingering/stale read before the game actually
// resets) clamps to the 40-40 display instead of a nonsensical code.
func pointDisplayCode(mine, other int, golden bool) int {
	if hasWonGamePadel(mine, other, gamePoints, golden) || hasWonGamePadel(other, mine, gamePoints, golden) {
		return 3
	}
	if mine <= 2 {
		return mine
	}
	// Past 40 (mine >= 3) and not yet decided: this can only be a deuce-and-
	// beyond state (other >= 3 too, see hasWonGamePadel's own 4-point floor).
	// Advantage is exactly a 1-point lead there; anything else at this stage
	// (mine == other, or a plain 40 against an opponent still below 40) is
	// the 40 display.
	if !golden && other >= 3 && mine == other+1 {
		return 4
	}
	return 3
}

// isTiebreak reports whether the current game being played is the
// deciding tiebreak of a set tied at Config.GamesPerSet-all.
func isTiebreak(s *State) bool {
	return setNeedsTiebreak(s.Games[0], s.Games[1], s.Cfg.GamesPerSet)
}

// gameTarget/gameMargin describe the point target for the game currently
// being played: the normal 4-point advantage game, or the tiebreakTarget
// once setNeedsTiebreak fires. GoldenPoint never applies inside a tiebreak
// (a tiebreak is already sudden-death-free scoring at 2-point margins).
func gamePointTarget(s *State) (target int, golden bool) {
	if isTiebreak(s) {
		return tiebreakTarget, false
	}
	return gamePoints, s.Cfg.GoldenPoint
}

// awardPointPadel is awardPoint's ScoringPadel branch (docs/design/
// contact-v2.md "Puntuación de pádel"): increments the current game's
// points (State.Score doubling as per-game points, see State.Games'
// comment), emits EvAce first when applicable, EvGamePoint/EvBreakPoint
// when the next point would decide the game, and -- once the game is
// actually won -- cascades into awardGamePadel for the game/set/match
// bookkeeping.
func awardPointPadel(s *State, winner int8, ev []Event) []Event {
	if s.Rally.ServeUntouched && winner == s.Server {
		ev = append(ev, Event{Kind: EvAce, Tick: s.Tick, Side: winner, Player: s.Rally.ServerPlayer})
	}

	s.Score[winner]++
	ev = append(ev, Event{Kind: EvPoint, Tick: s.Tick, Side: winner, Player: -1})

	target, golden := gamePointTarget(s)
	if hasWonGamePadel(s.Score[winner], s.Score[1-winner], target, golden) {
		return awardGamePadel(s, winner, ev)
	}
	rotateTiebreakServe(s)
	ev = appendGamePointWarnings(s, target, golden, ev)
	return startNextPoint(s, winner, ev)
}

// appendGamePointWarnings emits EvGamePoint (server) / EvBreakPoint
// (receiver) when the NEXT point, if won by that team, would finish the
// current game -- i.e. right after a point that brought the game to that
// edge, but before the game itself is decided.
func appendGamePointWarnings(s *State, target int, golden bool, ev []Event) []Event {
	for t := int8(0); t < 2; t++ {
		if !hasWonGamePadel(s.Score[t]+1, s.Score[1-t], target, golden) {
			continue
		}
		if t == s.Server {
			ev = append(ev, Event{Kind: EvGamePoint, Tick: s.Tick, Side: t, Player: -1})
		} else {
			ev = append(ev, Event{Kind: EvBreakPoint, Tick: s.Tick, Side: t, Player: -1})
		}
	}
	return ev
}

// rotateTiebreakServe applies the tiebreak serve order after a point that
// did not end it: the first point is served by the team due to serve, then
// the serve changes hands every 2 points (after points 1, 3, 5, ...). The
// box keeps alternating every point (serveBoxForScore on the tiebreak
// score). No-op outside a tiebreak.
func rotateTiebreakServe(s *State) {
	if isTiebreak(s) && (s.Score[0]+s.Score[1])%2 == 1 {
		s.Server = 1 - s.Server
	}
}

// nextServerAfterGame returns who serves the game after the one just won
// (called before Score resets): normally the other team; after a tiebreak,
// the team that RECEIVED its first point. s.Server is the server of the
// tiebreak's last point (index T-1), which is the first server F flipped
// once per completed 2-point block: F = Server xor ((T/2)&1).
func nextServerAfterGame(s *State, wasTiebreak bool) int8 {
	if !wasTiebreak {
		return 1 - s.Server
	}
	t := s.Score[0] + s.Score[1]
	first := s.Server ^ int8((t/2)&1)
	return 1 - first
}

// awardGamePadel resolves a finished game: emits EvGameWon, increments
// Games, flips the serving team (real padel always alternates service by
// GAME, never by who won the last point), resets the per-game points, then
// checks whether the set (and, cascading, the match) is also decided.
func awardGamePadel(s *State, winner int8, ev []Event) []Event {
	ev = append(ev, Event{Kind: EvGameWon, Tick: s.Tick, Side: winner, Player: -1})
	wasTiebreak := isTiebreak(s)
	s.Games[winner]++
	s.Server = nextServerAfterGame(s, wasTiebreak)
	s.Score = [2]int{}

	if wasTiebreak || hasWonSetGames(s.Games[winner], s.Games[1-winner], s.Cfg.GamesPerSet) {
		return awardSetPadel(s, winner, ev)
	}
	return startNextPoint(s, winner, ev)
}

// awardSetPadel resolves a finished set: emits EvSetWon, increments Sets,
// resets Games, then checks the match.
func awardSetPadel(s *State, winner int8, ev []Event) []Event {
	ev = append(ev, Event{Kind: EvSetWon, Tick: s.Tick, Side: winner, Player: -1})
	s.Sets[winner]++
	s.Games = [2]int{}

	if s.Sets[winner] >= s.Cfg.SetsToWin {
		s.Phase = PhaseGameOver
		return append(ev, Event{Kind: EvMatchWon, Tick: s.Tick, Side: winner, Player: -1})
	}
	return startNextPoint(s, winner, ev)
}

// startNextPoint begins the post-point pause (shared by both scoring
// modes): winner is only used by ScoringSimple's "winner serves next"
// legacy rule -- ScoringPadel already flipped State.Server in
// awardGamePadel, strict per-game alternation, before calling here.
func startNextPoint(s *State, winner int8, ev []Event) []Event {
	s.Phase = PhasePoint
	s.Rally.PauseTicks = PointPauseTicks
	_ = winner
	return ev
}
