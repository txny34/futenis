package sim

// Match owns the state and a reusable event buffer.
// Step mutates in place on purpose (hot path, zero allocations per tick).
type Match struct {
	state  State
	events []Event
}

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
	movePlayers(s, inputs)
	m.events = applyKicks(s, inputs, m.events)
	if s.Phase == PhaseRally {
		m.events = stepBall(s, m.events)
	}
	m.events = applyRules(s, m.events)
	return m.events
}
