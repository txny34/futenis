// Visible ranks from ELO (backlog "Retención": Bronce → Leyenda). Pure.
// Each tier but Leyenda has divisions III (low) → I (high), 50 ELO apiece:
// <800 Bronce III · 900 Plata III · 1000 Plata I · 1050 Oro III · 1499 Diamante I · 1500 Leyenda.
export type TierId = "bronze" | "silver" | "gold" | "platinum" | "diamond" | "legend";

interface Tier { readonly id: TierId; readonly name: string; readonly min: number }

const TIERS: readonly Tier[] = [
  { id: "bronze", name: "Bronce", min: 0 },
  { id: "silver", name: "Plata", min: 900 },
  { id: "gold", name: "Oro", min: 1050 },
  { id: "platinum", name: "Platino", min: 1200 },
  { id: "diamond", name: "Diamante", min: 1350 },
  { id: "legend", name: "Leyenda", min: 1500 },
];
const DIVISIONS = ["III", "II", "I"] as const;
const BRONZE_FLOOR = 750; // bronze spans 750..900 for its divisions (lower ratings stay Bronce III)

export interface Rank {
  readonly tier: TierId;
  /** "Oro II", "Leyenda". */
  readonly label: string;
  /** Ordinal for comparisons: tier * 3 + division (higher = better). */
  readonly order: number;
}

export function rankOf(elo: number): Rank {
  let t = 0;
  for (let i = TIERS.length - 1; i >= 0; i--) if (elo >= TIERS[i].min) { t = i; break; }
  const tier = TIERS[t];
  if (tier.id === "legend") return { tier: tier.id, label: tier.name, order: t * DIVISIONS.length };
  const lo = t === 0 ? BRONZE_FLOOR : tier.min;
  const span = (TIERS[t + 1].min - lo) / DIVISIONS.length;
  const d = Math.max(0, Math.min(DIVISIONS.length - 1, Math.floor((elo - lo) / span)));
  return { tier: tier.id, label: `${tier.name} ${DIVISIONS[d]}`, order: t * DIVISIONS.length + d };
}

export interface RankNote { readonly text: string; readonly up: boolean }

/** "¡Subiste a Oro I!" / "Bajaste a Plata I" / null when the rank did not change. */
export function rankChange(before: number, after: number): RankNote | null {
  const a = rankOf(before);
  const b = rankOf(after);
  if (a.order === b.order) return null;
  const up = b.order > a.order;
  return { text: up ? `¡Subiste a ${b.label}!` : `Bajaste a ${b.label}`, up };
}
