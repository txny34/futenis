import { LOBBY_PADS, LOBBY_PAD_RADIUS, LobbyCfg } from "./lobby/pads";
import { EventKind, Mode, Snap } from "./snapshot";

/** Buffer sizes the engine was built against (engine.ts). */
export interface CoreLayout {
  snapLen: number;
  inputPerPlayer: number;
  eventStride: number;
  maxEvents: number;
  cfgLen: number;
}

type ScalarKey = Exclude<keyof WasmLayout, "lobbyPads">;

/**
 * Compares this wasm build's buffer-layout constants (futenis.layout(),
 * wasm/layout.go) against the client's mirrored constants and throws a
 * descriptive error on any mismatch (surfaced as a HUD error by load()).
 * Without this, a client rebuilt against a newer/older sim would silently
 * read misaligned prev/curr snapshot slices instead of failing at load.
 */
export function assertLayout(bridge: FutenisBridge, core: CoreLayout): void {
  const got = bridge.layout?.();
  if (!got) return; // older wasm build predating layout(): nothing to check against
  const want: Required<Pick<WasmLayout, ScalarKey>> = {
    ...core,
    modeLobby: Mode.Lobby,
    snapLobbyPad: Snap.LobbyPad,
    snapLobbyHold: Snap.LobbyHold,
    evPadEnter: EventKind.PadEnter,
    evPadLeave: EventKind.PadLeave,
    evPadStart: EventKind.PadStart,
    cfgLobbyHoldTime: LobbyCfg.HoldTime,
    cfgLobbyHalfX: LobbyCfg.HalfX,
    cfgLobbyHalfZ: LobbyCfg.HalfZ,
    lobbyPadRadius: LOBBY_PAD_RADIUS,
  };
  const keys = Object.keys(want) as ScalarKey[];
  const detail = keys.filter((k) => got[k] !== want[k]).map((k) => `${k}: wasm=${got[k]} client=${want[k]}`);
  const pads = got.lobbyPads;
  const padsOk = pads !== undefined && pads.length === LOBBY_PADS.length && LOBBY_PADS.every((v, i) => pads[i] === v);
  if (!padsOk) detail.push(`lobbyPads: wasm=${pads ? pads.join(",") : "undefined"} client=${LOBBY_PADS.join(",")}`);
  if (detail.length === 0) return;
  throw new Error(`desalineado cliente/wasm (recompilá ambos): ${detail.join(", ")}`);
}
