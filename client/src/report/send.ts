import { accessToken } from "../auth/auth";
import { SUPABASE_KEY, SUPABASE_URL } from "../auth/config";

// Sends a bug report to public.bug_reports (migration 005: insert-only, the
// database stamps the account). Guests send with the publishable key alone.
const MAX_BODY = 12000;

export async function sendReport(body: string): Promise<void> {
  const token = accessToken();
  const headers: Record<string, string> = { apikey: SUPABASE_KEY, "Content-Type": "application/json", Prefer: "return=minimal" };
  if (token) headers.Authorization = `Bearer ${token}`;
  const res = await fetch(`${SUPABASE_URL}/rest/v1/bug_reports`, {
    method: "POST", headers, body: JSON.stringify({ body: body.slice(0, MAX_BODY) }),
  });
  if (!res.ok) throw new Error(`bug report ${res.status}`);
}
