package ai

import (
	"testing"

	"github.com/txny34/futenis/sim"
)

// serveLog counts serve outcomes over a bot-only padel match.
type serveLog struct {
	points, serverWins int
	volleys            int // receiving team touched the serve before its bounce (§21.6)
	faults, doubles    int
}

func (l *serveLog) record(s *sim.State, e sim.Event) {
	switch e.Kind {
	case sim.EvKick, sim.EvBodyHit:
		if s.Rally.ServeBoxPending && e.Side != s.Server {
			l.volleys++
		}
	case sim.EvServeFault:
		l.faults++
		if s.Rally.ServeAttempt >= 2 {
			l.doubles++
		}
	case sim.EvPoint:
		l.points++
		if e.Side == s.Server {
			l.serverWins++
		}
	}
}

// playServes runs bot-only padel matches (restarting at game over) for
// ticks and logs every serve. s is the state BEFORE the step that emitted e.
func playServes(mode sim.Mode, d Difficulty, ticks int) serveLog {
	cfg := sim.DefaultConfig()
	cfg.Mode = mode
	cfg.ScoringMode = sim.ScoringPadel
	m := sim.NewMatch(cfg)
	var bots [sim.MaxPlayers]Bot
	for i := range bots {
		bots[i] = New(d, i)
	}
	var log serveLog
	for t := 0; t < ticks; t++ {
		s := m.State()
		if s.Phase == sim.PhaseGameOver {
			m = sim.NewMatch(cfg)
			continue
		}
		var in [sim.MaxPlayers]sim.PlayerInput
		for i := 0; i < int(s.NumPlayers); i++ {
			in[i] = bots[i].Think(&s, i)
		}
		for _, e := range m.Step(in) {
			log.record(&s, e)
		}
	}
	return log
}

// The return of serve must wait for the bounce (§7.5, §21.6): no bot, at
// any difficulty, kicks or body-hits the serve before it lands.
func TestReceiverNeverTouchesServeBeforeBounce(t *testing.T) {
	for _, mode := range []sim.Mode{sim.Mode1v1, sim.Mode2v2} {
		for d := Easy; d <= Pro; d++ {
			l := playServes(mode, d, 300*sim.TickRate)
			if l.points < 15 {
				t.Errorf("mode %d %v: only %d points in 300 s", mode, d, l.points)
			}
			if l.volleys != 0 {
				t.Errorf("mode %d %v: receiver touched the serve before its bounce %d times (%d points)", mode, d, l.volleys, l.points)
			}
		}
	}
}
