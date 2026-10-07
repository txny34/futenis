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
	// ChargeError is the +/- error on the chosen charge fraction (0..1).
	ChargeError float64
	// MinSprintStamina is the stamina floor below which the bot stops
	// sprinting (stamina management: higher = more conservative).
	MinSprintStamina float64
	// NetMargin is the clearance (m) over the net tape a drive is charged
	// for: lower = faster, flatter, riskier shots.
	NetMargin float64
	// LineMargin is how far (m) inside the side/back glass the bot aims.
	LineMargin float64
	// DropChance is how often a smart shot is a short backspin drop when
	// the opponent is deep.
	DropChance float64
	// MissChance is 0..1, how often a shot is badly mistimed (released far
	// too early, a whiff). With the aim assist nobody hits the net or out,
	// so these misses are what end rallies between bots.
	MissChance float64
	// IdealErr is the contact error (0 = dead center, sim.Config's
	// QualPerfectMax = PERFECT) the bot plans its positioning for: it first
	// looks for a reachable contact tick whose height error is within
	// IdealErr and stands close enough to the ball's path to match; 0 = only
	// plan to reach the ball.
	IdealErr float64
	// UsesSpin / UsesJump enable spin selection and jumping for high balls.
	UsesSpin, UsesJump bool
	// ServeFault is 0..1, how often a serve is aimed outside the box (a
	// real fault: long or into the wrong box).
	ServeFault float64
	// ServeDepth is the mean serve depth as a fraction of ServiceLineDist.
	ServeDepth float64
	// ServeLine is how far (m) inside the box lines the serve may land:
	// lower = closer to the lines.
	ServeLine float64
	// ServeTossJitter is the max extra ticks the server waits before tossing.
	ServeTossJitter int
}

// levels are the presets indexed by Difficulty.
var levels = [difficultyCount]Level{
	Easy: {ReactionTicks: 30, AimNoise: 2.0, Decision: 0.3, PosError: 0.5, TimingError: 6, ChargeError: 0.35, MinSprintStamina: 0.0, NetMargin: 0.6, LineMargin: 1.5, DropChance: 0.05, MissChance: 0.22, IdealErr: 0.3,
		ServeFault: 0.095, ServeDepth: 0.55, ServeLine: 1.0, ServeTossJitter: 36},
	Normal: {ReactionTicks: 18, AimNoise: 1.2, Decision: 0.6, PosError: 0.3, TimingError: 3, ChargeError: 0.2, MinSprintStamina: 0.2, NetMargin: 0.5, LineMargin: 1.2, DropChance: 0.15, MissChance: 0.12, IdealErr: 0.12, UsesSpin: true,
		ServeFault: 0.055, ServeDepth: 0.65, ServeLine: 0.8, ServeTossJitter: 24},
	Hard: {ReactionTicks: 9, AimNoise: 0.6, Decision: 0.85, PosError: 0.25, TimingError: 1, ChargeError: 0.1, MinSprintStamina: 0.3, NetMargin: 0.35, LineMargin: 0.9, DropChance: 0.25, MissChance: 0.09, IdealErr: 0.08, UsesSpin: true, UsesJump: true,
		ServeFault: 0.02, ServeDepth: 0.75, ServeLine: 0.6, ServeTossJitter: 16},
	Pro: {ReactionTicks: 0, AimNoise: 0.25, Decision: 1.0, PosError: 0.15, TimingError: 0, ChargeError: 0, MinSprintStamina: 0.35, NetMargin: 0.2, LineMargin: 0.6, DropChance: 0.35, MissChance: 0.05, IdealErr: 0.06, UsesSpin: true, UsesJump: true,
		ServeFault: 0.01, ServeDepth: 0.8, ServeLine: 0.45, ServeTossJitter: 10},
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
