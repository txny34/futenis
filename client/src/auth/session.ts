import { SUPABASE_KEY, SUPABASE_URL, type Provider } from "./config";

// Login without SDK: Supabase Auth REST (implicit OAuth flow). The provider
// redirects back with the tokens in the URL hash; we keep them in localStorage.
const STORE_KEY = "futenis.auth";
const GUEST_KEY = "futenis.auth.guest";
const REFRESH_EARLY_S = 60;

export interface AuthUser {
  readonly id: string;
  readonly name: string;
  readonly avatar: string;
  readonly provider: string;
}

export interface Session {
  readonly accessToken: string;
  readonly refreshToken: string;
  /** Unix seconds. */
  readonly expiresAt: number;
  readonly user: AuthUser;
}

interface TokenReply { access_token?: string; refresh_token?: string; expires_in?: number }
interface UserReply {
  id?: string;
  app_metadata?: { provider?: string };
  user_metadata?: { full_name?: string; name?: string; custom_claims?: { global_name?: string }; avatar_url?: string };
}

const nowS = (): number => Math.floor(Date.now() / 1000);

function headers(token?: string): HeadersInit {
  const h: Record<string, string> = { apikey: SUPABASE_KEY, "Content-Type": "application/json" };
  if (token) h.Authorization = `Bearer ${token}`;
  return h;
}

function read<T>(key: string): T | null {
  try {
    const raw = localStorage.getItem(key);
    return raw ? (JSON.parse(raw) as T) : null;
  } catch { return null; }
}

function write(key: string, v: unknown): void {
  try {
    if (v === null) localStorage.removeItem(key);
    else localStorage.setItem(key, JSON.stringify(v));
  } catch { /* storage blocked: the session lasts this page only */ }
}

/** Leaves the page for the provider's consent screen; it returns to this page. */
export function signIn(provider: Provider): void {
  const back = `${location.origin}${location.pathname}`;
  location.assign(`${SUPABASE_URL}/auth/v1/authorize?provider=${provider}&redirect_to=${encodeURIComponent(back)}`);
}

export function chooseGuest(): void { write(GUEST_KEY, true); }
export const guestChosen = (): boolean => read<boolean>(GUEST_KEY) === true;

async function fetchUser(token: string): Promise<AuthUser> {
  const res = await fetch(`${SUPABASE_URL}/auth/v1/user`, { headers: headers(token) });
  if (!res.ok) throw new Error(`auth user ${res.status}`);
  const u = (await res.json()) as UserReply;
  const m = u.user_metadata ?? {};
  const name = m.custom_claims?.global_name || m.full_name || m.name || "";
  if (!u.id) throw new Error("auth user without id");
  return { id: u.id, name: name.slice(0, 24), avatar: m.avatar_url ?? "", provider: u.app_metadata?.provider ?? "" };
}

async function fromTokens(t: TokenReply): Promise<Session> {
  if (!t.access_token || !t.refresh_token || !t.expires_in) throw new Error("auth: incomplete tokens");
  const user = await fetchUser(t.access_token);
  return { accessToken: t.access_token, refreshToken: t.refresh_token, expiresAt: nowS() + t.expires_in, user };
}

/** Back from the provider: tokens (or an error) in the hash. Cleans the URL either way. */
async function consumeRedirect(): Promise<Session | null> {
  const h = new URLSearchParams(location.hash.slice(1));
  if (!h.has("access_token") && !h.has("error")) return null;
  history.replaceState(null, "", `${location.pathname}${location.search}`);
  const err = h.get("error_description") ?? h.get("error");
  if (err) throw new Error(err);
  return fromTokens({ access_token: h.get("access_token") ?? "", refresh_token: h.get("refresh_token") ?? "", expires_in: Number(h.get("expires_in")) });
}

async function refresh(s: Session): Promise<Session> {
  const res = await fetch(`${SUPABASE_URL}/auth/v1/token?grant_type=refresh_token`, {
    method: "POST", headers: headers(), body: JSON.stringify({ refresh_token: s.refreshToken }),
  });
  if (!res.ok) throw new Error(`auth refresh ${res.status}`);
  return fromTokens((await res.json()) as TokenReply);
}

/** Boot: redirect tokens > stored session (refreshed if near expiry) > none. */
export async function restoreSession(): Promise<Session | null> {
  const fresh = await consumeRedirect();
  let s = fresh ?? read<Session>(STORE_KEY);
  if (s && s.expiresAt - REFRESH_EARLY_S <= nowS()) {
    try { s = await refresh(s); } catch { s = null; }
  }
  write(STORE_KEY, s);
  return s;
}

export async function signOut(s: Session | null): Promise<void> {
  write(STORE_KEY, null);
  write(GUEST_KEY, null);
  if (!s) return;
  try { await fetch(`${SUPABASE_URL}/auth/v1/logout`, { method: "POST", headers: headers(s.accessToken) }); } catch { /* local logout is enough */ }
}
