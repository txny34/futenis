package sim

import "math"

// sanitizeInput guards the deterministic simulation against invalid client
// input: NaN collapses to 0, axes are clamped to their documented ranges.
func sanitizeInput(in PlayerInput) PlayerInput {
	return PlayerInput{
		MoveX: clampAxis(in.MoveX),
		MoveZ: clampAxis(in.MoveZ),
		Kick:  in.Kick,
		AimX:  clampAxis(in.AimX),
		AimZ:  clampAxis(in.AimZ),
		Loft:  clampUnit(in.Loft),
		Power: clampUnit(in.Power),
	}
}

// clampAxis sanitizes a -1..1 directional axis (MoveX/MoveZ/AimX/AimZ).
func clampAxis(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	return clamp(v, -1, 1)
}

// clampUnit sanitizes a 0..1 intensity value (Loft/Power).
func clampUnit(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	return clamp(v, 0, 1)
}
