package sim

import "math"

// Vec3 is a value type; all helpers return new values (no hidden aliasing).
type Vec3 struct{ X, Y, Z float64 }

func (a Vec3) Add(b Vec3) Vec3 { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func (a Vec3) Sub(b Vec3) Vec3 { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }

// Scale multiplies by s. Each component is a single op, so no FMA fusion risk.
func (a Vec3) Scale(s float64) Vec3 { return Vec3{a.X * s, a.Y * s, a.Z * s} }

// AddScaled returns a + b*s with explicit rounding to keep native and WASM bit-identical.
func (a Vec3) AddScaled(b Vec3, s float64) Vec3 {
	return Vec3{
		a.X + float64(b.X*s),
		a.Y + float64(b.Y*s),
		a.Z + float64(b.Z*s),
	}
}

func (a Vec3) Dot(b Vec3) float64 {
	return float64(a.X*b.X) + float64(a.Y*b.Y) + float64(a.Z*b.Z)
}

func (a Vec3) Len() float64 { return math.Sqrt(a.Dot(a)) }

// Normalized returns the unit vector, or zero for a zero vector.
func (a Vec3) Normalized() Vec3 {
	l := a.Len()
	if l == 0 {
		return Vec3{}
	}
	return a.Scale(1 / l)
}
