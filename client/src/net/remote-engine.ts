import { HumanInput, OUT_LEN, StepEngine, StepResult, emptyEvent, readEventsInto } from "../engine";
import { MAX_EVENTS, SNAP_LEN, SimEvent } from "../snapshot";
import { LagQueue, OwnBytes } from "./lag-queue";
import {
  CLOSE_GONE, CLOSE_REQUEUE, CLOSE_STATUS, NORMAL_CLOSE, REJOINABLE, REJOIN_EVERY_MS, REJOIN_WINDOW_MS, authMessage, rejoinToken, rejoinUrl, type NetStatus,
} from "./net-status";
import { HEADER, INPUT_MSG_LEN, Msg, PING_MSG_LEN, PROTO_VERSION, f64At, setF64At, writeHeader } from "./protocol";
import { StyleLink } from "./style-wire";
export type { NetStatus } from "./net-status";

const FRAME_BYTES = HEADER + OUT_LEN * 8;
const QUEUE_CAP = 256;
// Control messages (WELCOME, WAITING, PONG, PEER_LEFT) get their own queue so a
// backlog of frames (hidden tab: no rAF, no drain) can never push them out.
const CONTROL_CAP = 32;
const CONTROL_BYTES = HEADER + 2 * 8;
const CONTROL_SLOT = HEADER + 56; // largest control message: STYLES (4 x 14); SEAT is 32
const PING_EVERY_MS = 1000;
// Inputs are held levels: one per sim tick (120 Hz) is enough, whatever the display rate.
const INPUT_EVERY_MS = 1000 / 120;

/** StepEngine backed by the online server: inputs go up, server frames come down (optionally delayed). */
export class RemoteEngine implements StepEngine {
  onStatus: (s: NetStatus, pingMs: number) => void = () => undefined;
  /** Ranked: the server's ELO before/after the match (cmd/server/ranked.go RESULT). */
  onResult: (before: number, after: number) => void = () => undefined;
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
  private token = ""; // this seat's rejoin secret (SEAT)
  private retryUntil = 0; // end of the rejoin window (performance.now ms)
  private retryTimer = 0;
  private awayUntil = 0; // when the dropped rival's hold runs out
  /** Every seat's look (STYLES); set its onLooks. */
  readonly looks = new StyleLink((msg) => this.up.push(msg, performance.now()));

  /** auth: Supabase session token, sent as the first message (ranked queues only). */
  constructor(private readonly url: string, lagMs: number, private readonly auth = "") {
    this.up = new LagQueue(QUEUE_CAP, INPUT_MSG_LEN, lagMs / 2);
    this.down = new LagQueue(QUEUE_CAP, FRAME_BYTES, lagMs / 2);
    this.control = new LagQueue(CONTROL_CAP, CONTROL_SLOT, lagMs / 2);
    this.result = { prev: this.out.subarray(0, SNAP_LEN), curr: this.out.subarray(SNAP_LEN, 2 * SNAP_LEN), events: this.pool, eventCount: 0 };
    writeHeader(this.inputView, Msg.Input);
    writeHeader(this.pingView, Msg.Ping);
  }

  /** True while the socket is open: GameLoop must keep calling step() so WAITING/WELCOME get drained. */
  get loaded(): boolean { return this.ws?.readyState === WebSocket.OPEN; }

  connect(): void { this.setStatus("conectando"); this.open(this.url); }

  /** Seconds the dropped rival has left to come back ("rival reconectando"). */
  awaySeconds(): number { return Math.max(0, Math.ceil((this.awayUntil - performance.now()) / 1000)); }

  private open(url: string): void {
    const ws = new WebSocket(url);
    ws.binaryType = "arraybuffer";
    ws.onopen = () => { if (this.auth && url === this.url) ws.send(authMessage(this.auth)); };
    ws.onmessage = (m: MessageEvent<ArrayBuffer>) => {
      const b = new Uint8Array(m.data);
      (b[0] === Msg.Frame ? this.down : this.control).push(b, performance.now());
    };
    ws.onclose = (e: CloseEvent) => { if (this.ws === ws) this.closed(e.code); };
    this.ws = ws;
  }

  private closed(code: number): void {
    if (code === CLOSE_REQUEUE && this.status === "esperando rival") { this.open(this.url); return; } // same search, closer room
    const dropped = CLOSE_STATUS[code] === undefined && code !== CLOSE_GONE && code !== NORMAL_CLOSE;
    if (dropped && this.token !== "" && REJOINABLE.has(this.status)) { this.retry(); return; }
    if (this.status !== "esperando rival" && this.status !== "conectando" && !REJOINABLE.has(this.status)) return;
    this.setStatus(CLOSE_STATUS[code] ?? "desconectado");
  }

  /** Connection lost mid-match: try to take the held seat back until the window closes. */
  private retry(): void {
    const now = performance.now();
    if (this.status !== "reconectando") { this.retryUntil = now + REJOIN_WINDOW_MS; this.setStatus("reconectando"); }
    if (now > this.retryUntil) { this.setStatus("desconectado"); return; }
    this.retryTimer = window.setTimeout(() => this.open(rejoinUrl(this.url, this.token)), REJOIN_EVERY_MS);
  }

  /** Leaves the match for good: no status change, no reconnect. */
  close(): void {
    window.clearTimeout(this.retryTimer);
    this.looks.dispose();
    const ws = this.ws;
    this.ws = null;
    if (ws) { ws.onclose = null; ws.onmessage = null; ws.close(NORMAL_CLOSE); } // 1000: the server frees the seat now
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
          this.looks.welcome(msg[2]);
        }
        break;
      case Msg.Waiting: if (this.status === "conectando") this.setStatus("esperando rival"); break;
      case Msg.PeerLeft: this.setStatus("el rival se fue"); break;
      case Msg.Styles: this.looks.receive(msg, len); break;
      case Msg.Seat:
        this.token = rejoinToken(msg.subarray(HEADER, len)) ?? this.token;
        break;
      case Msg.PeerAway:
        if (len < HEADER + 8) break;
        this.awayUntil = performance.now() + f64At(view, 0) * 1000;
        this.setStatus("rival reconectando");
        break;
      case Msg.PeerBack: if (this.status === "rival reconectando") this.setStatus("jugando"); break;
      case Msg.Result:
        if (len >= CONTROL_BYTES) this.onResult(f64At(view, 0), f64At(view, 1));
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
