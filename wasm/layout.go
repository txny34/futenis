package main

import (
	"encoding/binary"
	"math"

	"github.com/txny34/futenis/sim"
)

// Buffer layout shared with the client (client/src mirrors it; keep EXACT).
// All values are float64 little-endian.
const (
	// inputPerPlayer is the float count per player: moveX, moveZ, kick, aimX, aimZ, loft, power.
	inputPerPlayer = 7
	// inputLen is the total float count of the input buffer (player 0 then player 1).
	inputLen = 2 * inputPerPlayer
	// maxEvents caps the events reported per step call; extra events are dropped.
	maxEvents = 32
	// eventStride is the float count per event: kind, tick, posX, posY, posZ, side.
	eventStride = 6
	// outPrev is the offset of the snapshot taken before the last tick of a call.
	outPrev = 0
	// outCurr is the offset of the snapshot taken after the last tick.
	outCurr = sim.SnapLen
	// outEventCount is the offset of the event count.
	outEventCount = 2 * sim.SnapLen
	// outEvents is the offset of the first event.
	outEvents = outEventCount + 1
	// outLen is the total float count of the output buffer.
	outLen = outEvents + maxEvents*eventStride
	// minSteps and maxSteps bound the ticks run per step call.
	minSteps = 1
	maxSteps = 15
)

// decodeInputs reads inputLen LE float64s from src. NaN/Inf become 0.
func decodeInputs(src []byte) [2]sim.PlayerInput {
	var in [2]sim.PlayerInput
	for p := range in {
		b := p * inputPerPlayer
		in[p] = sim.PlayerInput{
			MoveX: readF64(src, b),
			MoveZ: readF64(src, b+1),
			Kick:  readF64(src, b+2) != 0,
			AimX:  readF64(src, b+3),
			AimZ:  readF64(src, b+4),
			Loft:  readF64(src, b+5),
			Power: readF64(src, b+6),
		}
	}
	return in
}

func readF64(src []byte, i int) float64 {
	f := math.Float64frombits(binary.LittleEndian.Uint64(src[i*8:]))
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return f
}

// encodeFloats writes src as LE float64s into dst (len(dst) >= 8*len(src)).
func encodeFloats(dst []byte, src []float64) {
	for i, f := range src {
		binary.LittleEndian.PutUint64(dst[i*8:], math.Float64bits(f))
	}
}

// writeEvent stores event e in slot i of out (len outLen).
func writeEvent(out []float64, i int, e sim.Event) {
	o := outEvents + i*eventStride
	out[o] = float64(e.Kind)
	out[o+1] = float64(e.Tick)
	out[o+2] = e.Pos.X
	out[o+3] = e.Pos.Y
	out[o+4] = e.Pos.Z
	out[o+5] = float64(e.Side)
}

// runFrame advances m by n ticks (clamped) and fills out (len outLen) with
// prev/curr snapshots and up to maxEvents events. Kick only applies to the
// first tick (edge-triggered). Mutates out in place to avoid allocations.
func runFrame(m *sim.Match, inputs [2]sim.PlayerInput, n int, out []float64) {
	n = clamp(n, minSteps, maxSteps)
	count := 0
	for i := 0; i < n; i++ {
		if i == n-1 {
			st := m.State()
			sim.WriteSnapshot(&st, out[outPrev:outCurr])
		}
		for _, e := range m.Step(inputs) {
			if count < maxEvents {
				writeEvent(out, count, e)
				count++
			}
		}
		inputs[0].Kick, inputs[1].Kick = false, false
	}
	st := m.State()
	sim.WriteSnapshot(&st, out[outCurr:outEventCount])
	out[outEventCount] = float64(count)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
