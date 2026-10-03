// Globals provided by dist/wasm_exec.js and the Go WASM module.
declare class Go {
  importObject: WebAssembly.Imports;
  run(instance: WebAssembly.Instance): Promise<void>;
}

/** Buffer-layout constants this wasm build was compiled with (wasm/main.go
 * layoutFn). The client asserts its own constants match this at load time
 * instead of silently misaligning prev/curr snapshot slices when either
 * side appends a field and the other doesn't rebuild (see Engine.load). */
interface WasmLayout {
  snapLen: number;
  inputPerPlayer: number;
  eventStride: number;
  maxEvents: number;
  cfgLen: number;
}

interface FutenisBridge {
  init(inU8: Uint8Array, outU8: Uint8Array): string | null;
  step(n: number): string | null;
  reset(): void;
  /** mode 0=1v1 1=2v2 2=playground; difficulty 0..3; humanMask bit i = slot i driven by the client. */
  configure(mode: number, difficulty: number, pointsToWin: number, humanMask: number): string | null;
  respawnBall(): void;
  /** Live tuning (contact-v2 §13). Fills dst with CfgLen LE float64s, or returns a fresh Uint8Array. */
  getConfig(dst?: Uint8Array): Uint8Array | string | null | undefined;
  setConfig(src: Uint8Array): string | null | undefined;
  /** Optional: absent on an older wasm build predating the layout() export. */
  layout?(): WasmLayout;
}

declare var futenis: FutenisBridge | undefined;
