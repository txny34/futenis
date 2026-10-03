package main

import (
	"encoding/binary"
	"math"

	"github.com/txny34/futenis/sim"
)

// Buffer layout shared with the client (client/src mirrors it; keep EXACT).
// All values are float64 little-endian.
const (
	// inputPerPlayer is the float count per player: moveX, moveZ, targetX,
	// targetZ, buttons, spin (docs/design/contact-v2.md §11.3/§2 v2).
	inputPerPlayer = 6
	// inputLen is the total float count of the input buffer, one block per
	// sim.MaxPlayers slot (client fills the human slots; unused/AI slots
	// are overwritten inside wasm by ai.Bot, see aiDriver).
	inputLen = sim.MaxPlayers * inputPerPlayer
	// maxEvents caps the events reported per step call; extra events are dropped.
	maxEvents = 32
	// eventStride is the float count per event: kind, tick, posX, posY,
	// posZ, side, player, part, quality, velX, velY, velZ, spin, _pad.
	eventStride = 14
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
// PlayerInput v2 carries held levels (Buttons/Spin), not edges, so callers
// no longer need to clear anything between the ticks of a single step call
// (see runFrame).
func decodeInputs(src []byte) [sim.MaxPlayers]sim.PlayerInput {
	var in [sim.MaxPlayers]sim.PlayerInput
	for p := range in {
		b := p * inputPerPlayer
		in[p] = sim.PlayerInput{
			MoveX:   readF64(src, b),
			MoveZ:   readF64(src, b+1),
			TargetX: readF64(src, b+2),
			TargetZ: readF64(src, b+3),
			Buttons: uint8(readF64(src, b+4)),
			Spin:    int8(readF64(src, b+5)),
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
	out[o+6] = float64(e.Player)
	out[o+7] = float64(e.Part)
	out[o+8] = float64(e.Quality)
	out[o+9] = e.Vel.X
	out[o+10] = e.Vel.Y
	out[o+11] = e.Vel.Z
	out[o+12] = e.Spin
	out[o+13] = 0 // _pad, reserved
}

// runFrame advances m by n ticks (clamped) and fills out (len outLen) with
// prev/curr snapshots and up to maxEvents events. inputs are held levels
// (PlayerInput v2): unlike the old edge-triggered Kick, they are reused
// as-is for every tick of this call, no per-tick clearing needed. Mutates
// out in place to avoid allocations. drv (may be nil) overwrites the AI
// slots' inputs before every tick, so bots react tick by tick while the
// human inputs stay the ones copied in once per call.
func runFrame(m *sim.Match, inputs [sim.MaxPlayers]sim.PlayerInput, n int, out []float64, drv *aiDriver) {
	n = clamp(n, minSteps, maxSteps)
	count := 0
	for i := 0; i < n; i++ {
		if i == n-1 {
			st := m.State()
			sim.WriteSnapshot(&st, out[outPrev:outCurr])
		}
		drv.fill(m, &inputs)
		for _, e := range m.Step(inputs) {
			if count < maxEvents {
				writeEvent(out, count, e)
				count++
			}
		}
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
