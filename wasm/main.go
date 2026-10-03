//go:build js && wasm

// Command wasm is the only bridge between sim and the browser. It exposes a
// global `futenis` object (init, step, reset). Each frame does a single
// JS<->Go crossing: inputs are copied in, snapshots + events copied out,
// using pre-allocated package-level buffers. Layout lives in layout.go.
package main

import (
	"fmt"
	"syscall/js"

	"github.com/txny34/futenis/sim"
)

var (
	match     *sim.Match
	inU8      js.Value
	outU8     js.Value
	inBytes   [inputLen * 8]byte
	outBytes  [outLen * 8]byte
	outFloats [outLen]float64
)

func main() {
	match = sim.NewMatch(sim.DefaultConfig())
	api := js.Global().Get("Object").New()
	api.Set("init", js.FuncOf(initFn))
	api.Set("step", js.FuncOf(stepFn))
	api.Set("reset", js.FuncOf(resetFn))
	js.Global().Set("futenis", api)
	select {}
}

// initFn(inU8, outU8) returns null on success or an error string.
func initFn(_ js.Value, args []js.Value) any {
	if len(args) != 2 {
		return "futenis.init: expected (inU8, outU8)"
	}
	if got := args[0].Get("byteLength").Int(); got != len(inBytes) {
		return fmt.Sprintf("futenis.init: input buffer is %d bytes, want %d", got, len(inBytes))
	}
	if got := args[1].Get("byteLength").Int(); got != len(outBytes) {
		return fmt.Sprintf("futenis.init: output buffer is %d bytes, want %d", got, len(outBytes))
	}
	inU8, outU8 = args[0], args[1]
	return nil
}

func resetFn(_ js.Value, _ []js.Value) any {
	match = sim.NewMatch(sim.DefaultConfig())
	return nil
}

// stepFn(n) runs n ticks and publishes the output buffer; returns null or an error string.
func stepFn(_ js.Value, args []js.Value) any {
	if inU8.IsUndefined() || outU8.IsUndefined() {
		return "futenis.step: call init first"
	}
	n := minSteps
	if len(args) > 0 && args[0].Type() == js.TypeNumber {
		n = args[0].Int()
	}
	if got := js.CopyBytesToGo(inBytes[:], inU8); got != len(inBytes) {
		return fmt.Sprintf("futenis.step: copied %d input bytes, want %d", got, len(inBytes))
	}
	runFrame(match, decodeInputs(inBytes[:]), n, outFloats[:])
	encodeFloats(outBytes[:], outFloats[:])
	if got := js.CopyBytesToJS(outU8, outBytes[:]); got != len(outBytes) {
		return fmt.Sprintf("futenis.step: copied %d output bytes, want %d", got, len(outBytes))
	}
	return nil
}
