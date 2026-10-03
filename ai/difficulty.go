// Package ai drives sim players with the same sim.PlayerInput a human
// produces (docs/design/contact-v2.md §12). Dependency direction: ai -> sim;
// sim never imports ai. Everything here is deterministic (no math/rand, no
// time, no maps) and allocation-free per Think call, so the same bot code can
// run inside the wasm bridge and, later, on the authoritative server.
package ai

// Difficulty selects one of the predefined Level presets.
type Difficulty uint8

// Difficulty presets, from weakest to strongest. Values are part of the
// wasm JS API (futenis.configure's difficulty argument): never reorder.
const (
	Easy Difficulty = iota
	Normal
	Hard
	Pro
	difficultyCount
)

// Level is the tunable "skill" of a bot. All human-like errors are derived
// deterministically from sim.Hash64, never from math/rand.
type Level struct {
	// ReactionTicks is how many ticks the bot keeps acting on its stale
	// belief of the ball after the real ball diverges from it (a kick, an
	// unexpected bounce).
	ReactionTicks int
	// AimNoise is the max target deviation in meters (uniform +/-).
	AimNoise float64
	// Decision is 0..1, the probability of picking the tactically best
	// target (open corner / lob vs net player) instead of a random one.
	Decision float64
	// PosError is the max positioning error in meters at the intercept.
	PosError float64
	// TimingError is the max ticks the bot may release the strike early.
	TimingError int
	// ChargeTicks is how long the bot holds the strike before contact.
	ChargeTicks int
	// MinSprintStamina is the stamina floor below which the bot stops
	// sprinting (stamina management: higher = more conservative).
	MinSprintStamina float64
	// UsesSpin / UsesJump enable spin selection and jumping for high balls.
	UsesSpin, UsesJump bool
}

// levels are the presets indexed by Difficulty.
var levels = [difficultyCount]Level{
	Easy:   {ReactionTicks: 30, AimNoise: 2.0, Decision: 0.3, PosError: 0.45, TimingError: 6, ChargeTicks: 20, MinSprintStamina: 0.0},
	Normal: {ReactionTicks: 18, AimNoise: 1.2, Decision: 0.6, PosError: 0.3, TimingError: 3, ChargeTicks: 40, MinSprintStamina: 0.2, UsesSpin: true},
	Hard:   {ReactionTicks: 9, AimNoise: 0.6, Decision: 0.85, PosError: 0.15, TimingError: 1, ChargeTicks: 70, MinSprintStamina: 0.3, UsesSpin: true, UsesJump: true},
	Pro:    {ReactionTicks: 0, AimNoise: 0.25, Decision: 1.0, PosError: 0.0, TimingError: 0, ChargeTicks: 100, MinSprintStamina: 0.35, UsesSpin: true, UsesJump: true},
}

// LevelOf returns the preset for d (out-of-range values clamp to Pro).
func LevelOf(d Difficulty) Level {
	if d >= difficultyCount {
		d = Pro
	}
	return levels[d]
}

// String names the difficulty (test logs, debugging).
func (d Difficulty) String() string {
	switch d {
	case Easy:
		return "Easy"
	case Normal:
		return "Normal"
	case Hard:
		return "Hard"
	case Pro:
		return "Pro"
	}
	return "Difficulty(?)"
}

// Difficulty returns which preset Level matches b (Pro if customized).
func (b *Bot) Difficulty() Difficulty {
	for d := Easy; d < difficultyCount; d++ {
		if levels[d] == b.Level {
			return d
		}
	}
	return Pro
}
