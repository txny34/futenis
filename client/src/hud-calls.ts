import { EventKind, SimEvent } from "./snapshot";

// Referee calls from the event stream. EvPoint carries no reason, so the
// reason is inferred from the rally events that preceded it (client-side
// heuristic; a sim-side reason field would make this exact).
const TEAM_NAMES = ["Local", "Visita"] as const;
const REASON_OUT = "pelota afuera";
const REASON_NET = "red";
const REASON_WALL = "pared antes del pique";
const REASON_DOUBLE_BOUNCE = "doble pique";
const CALL_FAULT = "Falta";
const CALL_DOUBLE_FAULT = "Doble falta";
const CALL_LET = "Let — se repite el saque";
const FAULTS_PER_SERVE = 2;

export class CallTracker {
  private bounces = 0;
  private wallBeforeBounce = false;
  private lastKind = 0;
  private faults = 0;

  /** Returns a call to display, or null. Allocates a string only when a point/fault happens. */
  onEvent(e: SimEvent): string | null {
    switch (e.kind) {
      case EventKind.Kick:
      case EventKind.BodyHit:
        this.bounces = 0;
        this.wallBeforeBounce = false;
        break;
      case EventKind.BounceFloor:
        this.bounces++;
        break;
      case EventKind.HitWall:
        if (this.bounces === 0) this.wallBeforeBounce = true;
        break;
      case EventKind.ServeFault:
        return this.fault();
      case EventKind.Let:
        return CALL_LET;
      case EventKind.Point:
        return this.point(e.side);
      default:
        break;
    }
    if (e.kind !== EventKind.Jump && e.kind !== EventKind.Whiff) this.lastKind = e.kind;
    return null;
  }

  private fault(): string {
    this.faults++;
    if (this.faults >= FAULTS_PER_SERVE) {
      this.faults = 0;
      return CALL_DOUBLE_FAULT;
    }
    return CALL_FAULT;
  }

  private point(winner: number): string {
    const reason = this.reason();
    this.faults = 0;
    this.bounces = 0;
    this.wallBeforeBounce = false;
    this.lastKind = 0;
    const name = TEAM_NAMES[winner === 1 ? 1 : 0];
    return reason ? `Punto ${name} — ${reason}` : `Punto ${name}`;
  }

  private reason(): string {
    if (this.lastKind === EventKind.Out) return REASON_OUT;
    if (this.lastKind === EventKind.HitNet) return REASON_NET;
    if (this.wallBeforeBounce) return REASON_WALL;
    if (this.bounces >= 2) return REASON_DOUBLE_BOUNCE;
    return "";
  }
}
