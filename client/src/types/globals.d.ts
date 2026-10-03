// Globals provided by dist/wasm_exec.js and the Go WASM module.
declare class Go {
  importObject: WebAssembly.Imports;
  run(instance: WebAssembly.Instance): Promise<void>;
}

interface FutenisBridge {
  init(inU8: Uint8Array, outU8: Uint8Array): string | null;
  step(n: number): string | null;
  reset(): void;
}

declare var futenis: FutenisBridge | undefined;
