import { HumanInput, OUT_LEN, StepEngine, StepResult, emptyEvent, readEventsInto } from "../engine";
import { MAX_EVENTS, SNAP_LEN, SimEvent } from "../snapshot";
import { LagQueue, OwnBytes } from "./lag-queue";
import { HEADER, INPUT_MSG_LEN, Msg, PING_MSG_LEN, PROTO_VERSION, f64At, setF64At, writeHeader } from "./protocol";

const FRAME_BYTES = HEADER + OUT_LEN * 8;
const QUEUE_CAP = 256;
// Control messages (WELCOME, WAITING, PONG, PEER_LEFT) get their own queue so a
// backlog of frames (hidden tab: no rAF, no drain) can never push them out.
const CONTROL_CAP = 32;
const CONTROL_BYTES = HEADER + 2 * 8;
const PING_EVERY_MS = 1000;
// Inputs are held levels: one per sim tick (120 Hz) is enough, whatever the display rate.
const INPUT_EVERY_MS = 1000 / 120;

export type NetStatus =
  | "conectando"
  | "esperando rival"
  | "jugando"
  | "el rival se fue"
  | "desconectado"
  | "versión distinta, recargá"
  | "desconectado por inactividad"
  | "nadie se unió a la sala"
  | "demasiados mensajes";

// Server close codes (cmd/server/limits.go).
const CLOSE_STATUS: Readonly<Record<number, NetStatus | undefined>> = {
  4003: "desconectado por inactividad",
  4004: "demasiados mensajes",
  4005: "nadie se unió a la sala",
};

/** StepEngine backed by the online server: inputs go up, server frames come down (optionally delayed). */
export class RemoteEngine implements StepEngine {
  onStatus: (s: NetStatus, pingMs: number) => void = () => undefined;
  private ws: WebSocket | null = null;
  private status: NetStatus = "conectando";
  private ping = 0;
  private lastPingSentAt = -Infinity;
  private readonly out = new Float64Array(OUT_LEN);
  private readonly frameIn = new Float64Array(OUT_LEN);
  private readonly pool: SimEvent[] = Array.from({ length: MAX_EVENTS }, emptyEvent);
  private readonly result: StepResult;
  private readonly up: LagQueue;
  private readonly down: LagQueue;
  private readonly control: LagQueue;
  private readonly inputBuf = new Uint8Array(INPUT_MSG_LEN);
  private readonly inputView = new DataView(this.inputBuf.buffer);
  private readonly pingBuf = new Uint8Array(PING_MSG_LEN);
  private readonly pingView = new DataView(this.pingBuf.buffer);
  private lastInputAt = -Infinity;
  private pending = 0; // events accumulated since the last step

  constructor(private readonly url: string, lagMs: number) {
    this.up = new LagQueue(QUEUE_CAP, INPUT_MSG_LEN, lagMs / 2);
    this.down = new LagQueue(QUEUE_CAP, FRAME_BYTES, lagMs / 2);
    this.control = new LagQueue(CONTROL_CAP, CONTROL_BYTES, lagMs / 2);
    this.result = {
      prev: this.out.subarray(0, SNAP_LEN),
      curr: this.out.subarray(SNAP_LEN, 2 * SNAP_LEN),
      events: this.pool,
      eventCount: 0,
    };
    writeHeader(this.inputView, Msg.Input);
    writeHeader(this.pingView, Msg.Ping);
  }

  /** True while the socket is open: GameLoop must keep calling step() so WAITING/WELCOME get drained. */
  get loaded(): boolean { return this.ws?.readyState === WebSocket.OPEN; }

  connect(): void {
    this.setStatus("conectando");
    const ws = new WebSocket(this.url);
    ws.binaryType = "arraybuffer";
    ws.onmessage = (m: MessageEvent<ArrayBuffer>) => {
      const b = new Uint8Array(m.data);
      (b[0] === Msg.Frame ? this.down : this.control).push(b, performance.now());
    };
    ws.onclose = (e: CloseEvent) => {
      if (this.status !== "jugando" && this.status !== "esperando rival" && this.status !== "conectando") return;
      this.setStatus(CLOSE_STATUS[e.code] ?? "desconectado");
    };
    this.ws = ws;
  }

  /** Leaves the match for good: no status change, no reconnect. */
  close(): void {
    const ws = this.ws;
    this.ws = null;
    if (ws) { ws.onclose = null; ws.onmessage = null; ws.close(); }
  }

  peek(): StepResult { this.result.eventCount = 0; return this.result; }

  /** Only slot 0 exists for the client (the server rotates the guest's view). */
  setInput(slot: number, a: HumanInput): void {
    const now = performance.now();
    if (slot !== 0 || now - this.lastInputAt < INPUT_EVERY_MS) return;
    this.lastInputAt = now;
    const v = this.inputView;
    setF64At(v, 0, a.moveX); setF64At(v, 1, a.moveZ); setF64At(v, 2, a.targetX);
    setF64At(v, 3, a.targetZ); setF64At(v, 4, a.buttons); setF64At(v, 5, a.spin);
    this.up.push(this.inputBuf, now);
  }

  /** n is ignored: the server owns the clock. Returns the newest delivered frame and its events. */
  step(_n: number): StepResult {
    const now = performance.now();
    this.pumpPing(now);
    this.up.drain(now, this.sendUp);
    this.pending = 0;
    this.result.eventCount = 0;
    this.control.drain(now, this.receive);
    this.down.drain(now, this.receive);
    return this.result;
  }

  private readonly sendUp = (msg: OwnBytes, len: number): void => {
    if (this.ws?.readyState === WebSocket.OPEN) this.ws.send(msg.subarray(0, len));
  };

  private readonly receive = (msg: OwnBytes, len: number, view: DataView): void => {
    if (len < HEADER || msg[1] !== PROTO_VERSION) return;
    switch (msg[0]) {
      case Msg.Welcome:
        if (len < CONTROL_BYTES) break;
        if (f64At(view, 0) !== SNAP_LEN || f64At(view, 1) !== OUT_LEN) {
          this.setStatus("versión distinta, recargá");
          this.ws?.close();
        } else {
          this.setStatus("jugando");
        }
        break;
      case Msg.Waiting:
        if (this.status === "conectando") this.setStatus("esperando rival");
        break;
      case Msg.PeerLeft:
        this.setStatus("el rival se fue");
        break;
      case Msg.Pong:
        if (len < PING_MSG_LEN) break;
        this.ping = Math.round(performance.now() - f64At(view, 0));
        this.onStatus(this.status, this.ping);
        break;
      case Msg.Frame:
        if (len === FRAME_BYTES) this.receiveFrame(view);
        break;
      default:
        break;
    }
  };

  private receiveFrame(view: DataView): void {
    for (let i = 0; i < OUT_LEN; i++) this.frameIn[i] = view.getFloat64(HEADER + i * 8, true);
    this.out.set(this.frameIn);
    this.appendEvents();
  }

  /** Appends the frame's events after any already pending this step, up to MAX_EVENTS. */
  private appendEvents(): void {
    readEventsInto(this.frameIn, this.pool, this.result, this.pending);
    this.pending = this.result.eventCount;
  }

  private pumpPing(now: number): void {
    if (this.ws?.readyState !== WebSocket.OPEN || now - this.lastPingSentAt < PING_EVERY_MS) return;
    this.lastPingSentAt = now;
    setF64At(this.pingView, 0, now);
    this.up.push(this.pingBuf, now);
  }

  private setStatus(s: NetStatus): void {
    this.status = s;
    this.onStatus(this.status, this.ping);
  }
}
