package sim

// newPadelTestState builds a fresh ScoringPadel rules-test state, mirroring
// newRulesTestState (rules_test.go) but opted into the real scoring model.
func newPadelTestState() State {
	s := newRulesTestState()
	s.Cfg.ScoringMode = ScoringPadel
	s.Cfg.GamesPerSet = 3
	s.Cfg.SetsToWin = 1
	return s
}
