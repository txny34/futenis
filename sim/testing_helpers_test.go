package sim

import (
	"math"
	"testing"
)

// epsilon is the tolerance used when comparing floats produced by the
// simulation; tests never rely on exact binary equality for derived values.
const epsilon = 1e-9

// almostEqual reports whether a and b differ by no more than epsilon.
func almostEqual(t *testing.T, a, b float64) bool {
	t.Helper()
	return math.Abs(a-b) <= epsilon
}

// assertFloat fails the test with a "got X, want Y" message if got and want
// differ by more than epsilon.
func assertFloat(t *testing.T, name string, got, want float64) {
	t.Helper()
	if !almostEqual(t, got, want) {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}

// lcg is a tiny deterministic linear-congruential generator used to script
// reproducible pseudo-random inputs in tests, instead of math/rand, so the
// determinism tests never depend on an external seed source.
type lcg struct{ state uint64 }

// newLCG returns an lcg seeded with the given value.
func newLCG(seed uint64) *lcg { return &lcg{state: seed} }

// next advances the generator and returns the next raw 64-bit value.
func (g *lcg) next() uint64 {
	// Constants from Numerical Recipes.
	g.state = g.state*6364136223846793005 + 1442695040888963407
	return g.state
}

// floatRange returns a deterministic float64 in [lo, hi).
func (g *lcg) floatRange(lo, hi float64) float64 {
	frac := float64(g.next()>>11) / float64(1<<53)
	return lo + frac*(hi-lo)
}

// bool returns a deterministic boolean, true roughly 1/n of the time.
func (g *lcg) bool(n uint64) bool {
	if n == 0 {
		n = 1
	}
	return g.next()%n == 0
}

// scriptedInput builds a deterministic PlayerInput from the generator, used
// to drive long determinism runs without any real randomness.
func scriptedInput(g *lcg) PlayerInput {
	return PlayerInput{
		MoveX: g.floatRange(-1, 1),
		MoveZ: g.floatRange(-1, 1),
		Kick:  g.bool(5),
		AimX:  g.floatRange(-1, 1),
		AimZ:  g.floatRange(-1, 1),
		Loft:  g.floatRange(0, 1),
		Power: g.floatRange(0, 1),
	}
}
