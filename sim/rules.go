package sim

// OWNER: rules agent. Stubs define the contract; replace the bodies.

// applyRules reads the events emitted this tick, updates RallyState and Score,
// and appends EvPoint / EvGameOver when needed.
func applyRules(s *State, ev []Event) []Event { return ev }

// resetForServe places players and ball for a new serve by side `server`.
func resetForServe(s *State, server int8) {
	s.Server = server
	s.Phase = PhaseServe
}
