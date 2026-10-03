package sim

// Match owns the state and a reusable event buffer.
// Step mutates in place on purpose (hot path, zero allocations per tick).
type Match struct {
	state  State
	events []Event
}

// NewMatch creates a Match with the given Config, starting with side 0
// serving.
func NewMatch(cfg Config) *Match {
	m := &Match{events: make([]Event, 0, 32)}
	m.state.Cfg = cfg
	resetForServe(&m.state, 0)
	return m
}

// State returns a copy of the current state.
func (m *Match) State() State { return m.state }

// Restore replaces the state (used for rollback / server correction).
func (m *Match) Restore(s State) { m.state = s }

// Step advances exactly one fixed tick. The returned slice is only valid until the next Step.
func (m *Match) Step(inputs [2]PlayerInput) []Event {
	s := &m.state
	m.events = m.events[:0]
	s.Tick++

	if s.Phase == PhaseGameOver {
		return m.events
	}

	sanitized := [2]PlayerInput{sanitizeInput(inputs[0]), sanitizeInput(inputs[1])}

	movePlayers(s, sanitized)
	m.events = applyKicks(s, sanitized, m.events)
	// The ball keeps flying visually during the point pause; rules.go only
	// judges events while Phase == PhaseRally.
	if s.Phase == PhaseRally || s.Phase == PhasePoint {
		m.events = stepBall(s, m.events)
	}
	m.events = applyRules(s, m.events)
	return m.events
}
