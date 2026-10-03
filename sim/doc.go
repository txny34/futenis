// Package sim implements futenis' deterministic game rules and physics.
//
// # Determinism
//
// Given the same State and the same sequence of PlayerInput, Match.Step must
// always produce the same resulting State and Events, whether running
// natively or compiled to WASM. To keep that guarantee:
//
//   - No wall-clock time (time.Now), no unseeded randomness (math/rand), no
//     goroutines and no map iteration anywhere in this package.
//   - Multi-operation floating point expressions are wrapped in float64(...)
//     so the compiler cannot fuse them into an FMA instruction on arm64,
//     which would otherwise round differently than amd64/WASM.
//   - PlayerInput is sanitized (NaN -> 0, axes clamped) before it can
//     influence State, so malformed client input cannot diverge the
//     simulation or panic it.
//
// # Zero allocations per tick
//
// Match.Step must not allocate. The reusable Match.events buffer is reset
// with events[:0] instead of being reallocated, and State is a plain value
// type that callers copy explicitly via Match.State/Match.Restore.
package sim
