package sim

// Match owns the state and a reusable event buffer.
// Step mutates in place on purpose (hot path, zero allocations per tick).
type Match struct {
	state  State
	events []Event
}

// NewMatch creates a Match with the given Config. Mode1v1/Mode2v2 start
// with team 0 serving (PhaseServe); ModePlayground starts directly in
// PhaseRally with no scoring (docs/design/contact-v2.md "Modos y 2v2").
func NewMatch(cfg Config) *Match {
	m := &Match{events: make([]Event, 0, 32)}
	m.state.Cfg = cfg
	m.state.NumPlayers = numPlayersForMode(cfg.Mode)
	m.state.LastServerPlayer = [2]int8{-1, -1}
	if cfg.Mode == ModePlayground {
		setupPlayground(&m.state)
	} else {
		resetForServe(&m.state, cfg.FirstServer)
	}
	return m
}

// State returns a copy of the current state.
func (m *Match) State() State { return m.state }

// Restore replaces the state (used for rollback / server correction).
func (m *Match) Restore(s State) { m.state = s }

// RespawnBall resets the ball to a neutral position above center court.
// ModePlayground only: without scoring, the ball can be respawned on
// demand (e.g. a dev-panel button) instead of via the serve rules.
//
// TODO(wave1): wire an actual client/dev-panel trigger; Wave 0 only
// provides the primitive (docs/design/contact-v2.md "Modos y 2v2").
func (m *Match) RespawnBall() {
	m.state.Ball = Ball{Pos: Vec3{Y: serveBallHeight}}
}

// Step advances exactly one fixed tick. The returned slice is only valid
// until the next Step. inputs has MaxPlayers slots; only the first
// s.NumPlayers are read (see State.NumPlayers, numPlayersForMode).
func (m *Match) Step(inputs [MaxPlayers]PlayerInput) []Event {
	s := &m.state
	m.events = m.events[:0]
	s.Tick++

	if s.Phase == PhaseGameOver {
		return m.events
	}

	var sanitized [MaxPlayers]PlayerInput
	for i := int8(0); i < s.NumPlayers; i++ {
		sanitized[i] = sanitizeInput(inputs[i], s.Players[i].Team, &s.Cfg)
	}

	m.events = movePlayers(s, sanitized, m.events)
	m.events = applyStrikes(s, sanitized, m.events)
	// The ball keeps flying visually during the point pause; rules.go only
	// judges events while Phase == PhaseRally. wave1-C: once the serve has
	// been dropped (Ball.Tossed), it must also fall/bounce under real
	// physics while still Phase == PhaseServe, or the mandatory toss bounce
	// (docs/design/contact-v2.md §7.5) could never happen.
	if s.Phase == PhaseRally || s.Phase == PhasePoint || (s.Phase == PhaseServe && s.Ball.Tossed) {
		m.events = stepBall(s, m.events)
		m.events = applyBodyCollisions(s, m.events)
	}
	m.events = applyRules(s, m.events)
	return m.events
}
