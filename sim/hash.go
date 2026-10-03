package sim

import (
	"math"
	"math/bits"
)

// splitmix64 is the splitmix64 mix function: deterministic, carries no
// hidden state, bit-identical across amd64/arm64/wasm. Used instead of
// math/rand anywhere sim needs cheap pseudo-random-looking but reproducible
// bits (contact deviation in strike.go, the ai package's "human error").
func splitmix64(x uint64) uint64 {
	x += 0x9E3779B97F4A7C15
	x = (x ^ (x >> 30)) * 0xBF58476D1CE4E5B9
	x = (x ^ (x >> 27)) * 0x94D049BB133111EB
	return x ^ (x >> 31)
}

// Hash64 is splitmix64 exported for the ai package's deterministic "human
// error" model (seeded by tick), per docs/design/contact-v2.md §12.
func Hash64(x uint64) uint64 { return splitmix64(x) }

// strikeHash derives a per-tick, per-side, per-ball-position hash used to
// make contact deviation deterministic without math/rand
// (docs/design/contact-v2.md §8).
//
// TODO(wave1): wired into strike.go's deviateTarget/shotSpeed.
func strikeHash(tick uint32, side int8, b Vec3) uint64 {
	x := uint64(tick) | uint64(uint8(side))<<32
	x ^= math.Float64bits(b.X) ^ bits.RotateLeft64(math.Float64bits(b.Y), 21) ^
		bits.RotateLeft64(math.Float64bits(b.Z), 42)
	return splitmix64(x)
}

// unitPair returns two independent uniform floats in [-1, 1) derived from
// h (docs/design/contact-v2.md §8).
//
// TODO(wave1): consumed by deviateTarget.
func unitPair(h uint64) (a, b float64) {
	a = float64(h>>11)*0x1p-52 - 1
	h2 := splitmix64(h)
	b = float64(h2>>11)*0x1p-52 - 1
	return a, b
}
