import { SUPABASE_KEY, SUPABASE_URL } from "../../auth/config";

// Global top lists for the hall boards: public.ratings (public read; written
// only by the game server, names filtered there). Only players with ranked
// games are listed. null = not available (offline, or the database has no
// leaderboard columns yet): the board keeps its "coming soon" lines.
export type TopKind = "1v1" | "2v2" | "wins";
export interface TopRow { readonly id: string; readonly name: string; readonly value: number }

export const TOP_SIZE = 7;
const NAME_MAX = 24;
const QUERY: Readonly<Record<TopKind, { col: string; filter: string }>> = {
  "1v1": { col: "elo_1v1", filter: "games_1v1=gt.0" },
  "2v2": { col: "elo_2v2", filter: "games_2v2=gt.0" },
  wins: { col: "wins", filter: "wins=gt.0" },
};

function row(raw: unknown, col: string): TopRow | null {
  if (typeof raw !== "object" || raw === null) return null;
  const r = raw as Record<string, unknown>;
  const v = r[col];
  if (typeof r.user_id !== "string" || typeof v !== "number" || !Number.isFinite(v)) return null;
  const name = typeof r.name === "string" && r.name.trim() !== "" ? r.name.slice(0, NAME_MAX) : "Jugador";
  return { id: r.user_id, name, value: Math.round(v) };
}

export async function fetchTop(kind: TopKind): Promise<TopRow[] | null> {
  const { col, filter } = QUERY[kind];
  try {
    const url = `${SUPABASE_URL}/rest/v1/ratings?select=user_id,name,${col}&${filter}&order=${col}.desc,user_id.asc&limit=${TOP_SIZE}`;
    const res = await fetch(url, { headers: { apikey: SUPABASE_KEY } });
    if (!res.ok) return null;
    const raw: unknown = await res.json();
    if (!Array.isArray(raw)) return null;
    return raw.map((r) => row(r, col)).filter((r): r is TopRow => r !== null);
  } catch {
    return null;
  }
}
