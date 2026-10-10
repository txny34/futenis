import { profileWriter } from "../profile/store";
import { SUPABASE_KEY, SUPABASE_URL } from "./config";

// Ranked ELO lives in Supabase (table public.ratings, public read, written only
// by the game server). The profile shows it while signed in.
interface RatingRow { elo_1v1?: number; elo_2v2?: number }

const UUID = /^[0-9a-f-]{36}$/;

/** Copies the account's ranked ELO into the profile. Missing row = never played (keeps the default). */
export async function syncRatings(userId: string): Promise<void> {
  if (!UUID.test(userId)) return;
  const res = await fetch(`${SUPABASE_URL}/rest/v1/ratings?select=elo_1v1,elo_2v2&user_id=eq.${userId}`, { headers: { apikey: SUPABASE_KEY } });
  if (!res.ok) return;
  const rows = (await res.json()) as RatingRow[];
  const r = rows[0];
  if (!r) return;
  profileWriter().update((p) => ({
    ...p,
    elo: { "1v1": typeof r.elo_1v1 === "number" ? r.elo_1v1 : p.elo["1v1"], "2v2": typeof r.elo_2v2 === "number" ? r.elo_2v2 : p.elo["2v2"] },
  }));
}
