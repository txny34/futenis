//go:build js && wasm

// Command wasm is the only bridge between sim and the browser. It exposes a
// global `futenis` object (init, step, reset, configure, respawnBall,
// getConfig, setConfig). Each frame does a single JS<->Go crossing: inputs
// are copied in, snapshots + events copied out, using pre-allocated
// package-level buffers. Bots (ai.Bot) for non-human slots run inside step,
// before every sim tick. Layout lives in layout.go; session logic in
// session.go.
package main

import (
	"fmt"
	"syscall/js"

	"github.com/txny34/futenis/sim"
)

var (
	sess      session
	inU8      js.Value
	outU8     js.Value
	inBytes   [inputLen * 8]byte
	outBytes  [outLen * 8]byte
	outFloats [outLen]float64
	cfgBuf    [cfgBytes]byte
	cfgFloats [sim.CfgLen]float64
)

func main() {
	sess = defaultSession()
	api := js.Global().Get("Object").New()
	api.Set("init", js.FuncOf(initFn))
	api.Set("step", js.FuncOf(stepFn))
	api.Set("reset", js.FuncOf(resetFn))
	api.Set("configure", js.FuncOf(configureFn))
	api.Set("respawnBall", js.FuncOf(respawnBallFn))
	api.Set("getConfig", js.FuncOf(getConfigFn))
	api.Set("setConfig", js.FuncOf(setConfigFn))
	api.Set("configLen", sim.CfgLen)
	api.Set("layout", js.FuncOf(layoutFn))
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
	sess.reset()
	return nil
}

// configureFn(mode, difficulty, pointsToWin, humanMask) -> null | error string.
func configureFn(_ js.Value, args []js.Value) any {
	if len(args) != 4 {
		return "futenis.configure: expected (mode, difficulty, pointsToWin, humanMask)"
	}
	for i, a := range args {
		if a.Type() != js.TypeNumber {
			return fmt.Sprintf("futenis.configure: argument %d is not a number", i)
		}
	}
	if err := sess.configure(args[0].Int(), args[1].Int(), args[2].Int(), args[3].Int()); err != nil {
		return err.Error()
	}
	return nil
}

// respawnBallFn() -> null | error string (playground only).
func respawnBallFn(_ js.Value, _ []js.Value) any {
	if err := sess.respawnBall(); err != nil {
		return err.Error()
	}
	return nil
}

// getConfigFn(u8) fills a Uint8Array of configLen*8 bytes (LE float64s,
// sim/config_layout.go order) -> null | error string.
func getConfigFn(_ js.Value, args []js.Value) any {
	if errMsg := checkCfgArg("getConfig", args); errMsg != "" {
		return errMsg
	}
	sess.writeConfig(cfgBuf[:], cfgFloats[:])
	js.CopyBytesToJS(args[0], cfgBuf[:])
	return nil
}

// setConfigFn(u8) applies a config buffer between frames -> null | error string.
func setConfigFn(_ js.Value, args []js.Value) any {
	if errMsg := checkCfgArg("setConfig", args); errMsg != "" {
		return errMsg
	}
	js.CopyBytesToGo(cfgBuf[:], args[0])
	if err := sess.readConfig(cfgBuf[:], cfgFloats[:]); err != nil {
		return err.Error()
	}
	return nil
}

func checkCfgArg(name string, args []js.Value) string {
	if len(args) != 1 || args[0].Type() != js.TypeObject {
		return fmt.Sprintf("futenis.%s: expected (Uint8Array of %d bytes)", name, cfgBytes)
	}
	if got := args[0].Get("byteLength").Int(); got != cfgBytes {
		return fmt.Sprintf("futenis.%s: buffer is %d bytes, want %d", name, got, cfgBytes)
	}
	return ""
}

// layoutFn() -> {snapLen, inputPerPlayer, eventStride, maxEvents, cfgLen}:
// this build's buffer-layout constants, so the client can assert its own
// mirrored constants (client/src/snapshot.ts, engine.ts) match this exact
// build and refuse to run misaligned instead of silently reading garbage
// out of a stale prev/curr snapshot slice the moment either side appends a
// field and the other doesn't get rebuilt in step.
func layoutFn(_ js.Value, _ []js.Value) any {
	obj := js.Global().Get("Object").New()
	obj.Set("snapLen", sim.SnapLen)
	obj.Set("inputPerPlayer", inputPerPlayer)
	obj.Set("eventStride", eventStride)
	obj.Set("maxEvents", maxEvents)
	obj.Set("cfgLen", sim.CfgLen)
	return obj
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
	runFrame(sess.match, decodeInputs(inBytes[:]), n, outFloats[:], &sess.drv)
	encodeFloats(outBytes[:], outFloats[:])
	if got := js.CopyBytesToJS(outU8, outBytes[:]); got != len(outBytes) {
		return fmt.Sprintf("futenis.step: copied %d output bytes, want %d", got, len(outBytes))
	}
	return nil
}
