import { accessToken, logout } from "./auth";
import { SUPABASE_KEY, SUPABASE_URL } from "./config";

// Deletes the signed-in account for good (migration 004: delete_my_account drops
// the auth user; ratings and cloud progress go with it), then signs out here.
export async function deleteAccount(): Promise<void> {
  const token = accessToken();
  if (!token) throw new Error("delete account: not signed in");
  const res = await fetch(`${SUPABASE_URL}/rest/v1/rpc/delete_my_account`, {
    method: "POST",
    headers: { apikey: SUPABASE_KEY, Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: "{}",
  });
  if (!res.ok) throw new Error(`delete account ${res.status}`);
  await logout();
}
