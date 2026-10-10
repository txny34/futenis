import { SUPABASE_KEY, SUPABASE_URL, type Provider } from "./config";

// Login without SDK: Supabase Auth REST with PKCE. Before leaving for the
// provider we keep a random verifier in this tab; the return carries a one-use
// ?code= that only that verifier can redeem. A crafted link with someone else's
// tokens or code is never accepted. Tokens are kept in localStorage.
const STORE_KEY = "futenis.auth";
const VERIFIER_KEY = "futenis.auth.pkce";
const VERIFIER_BYTES = 48;
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

/** The auth server refused the session (revoked/expired refresh token): sign out. */
class AuthRejected extends Error {}

function isSession(v: unknown): v is Session {
  const s = v as Partial<Session> | null;
  return !!s && typeof s.accessToken === "string" && typeof s.refreshToken === "string"
    && typeof s.expiresAt === "number" && typeof s.user?.id === "string" && typeof s.user.name === "string";
}

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

const b64url = (b: Uint8Array): string =>
  btoa(String.fromCharCode(...b)).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");

async function challengeFor(verifier: string): Promise<string> {
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(verifier));
  return b64url(new Uint8Array(digest));
}

/** Leaves the page for the provider's consent screen; it returns to this page with ?code=. */
export async function signIn(provider: Provider): Promise<void> {
  const verifier = b64url(crypto.getRandomValues(new Uint8Array(VERIFIER_BYTES)));
  try { sessionStorage.setItem(VERIFIER_KEY, verifier); } catch { return; } // no storage: PKCE impossible
  const back = `${location.origin}${location.pathname}`;
  const q = new URLSearchParams({ provider, redirect_to: back, code_challenge: await challengeFor(verifier), code_challenge_method: "s256" });
  location.assign(`${SUPABASE_URL}/auth/v1/authorize?${q.toString()}`);
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

function takeVerifier(): string | null {
  try {
    const v = sessionStorage.getItem(VERIFIER_KEY);
    sessionStorage.removeItem(VERIFIER_KEY);
    return v;
  } catch { return null; }
}

/** Back from the provider: ?code= (or ?error=) redeemed only with this tab's verifier.
 * Cleans the URL either way; tokens in a hash are ignored (no implicit flow). */
async function consumeRedirect(): Promise<Session | null> {
  const q = new URLSearchParams(location.search);
  const hash = new URLSearchParams(location.hash.slice(1));
  const code = q.get("code");
  const failed = q.has("error") || hash.has("error");
  if (!code && !failed && !hash.has("access_token")) return null;
  history.replaceState(null, "", location.pathname);
  const verifier = takeVerifier();
  if (failed) throw new Error("login cancelled or rejected"); // fixed text: never echo the URL
  if (!code || !verifier) return null; // not a login this tab started
  const res = await fetch(`${SUPABASE_URL}/auth/v1/token?grant_type=pkce`, {
    method: "POST", headers: headers(), body: JSON.stringify({ auth_code: code, code_verifier: verifier }),
  });
  if (!res.ok) throw new Error(`auth code ${res.status}`);
  return fromTokens((await res.json()) as TokenReply);
}

/** New tokens for s. Throws AuthRejected when the server refuses it (sign out),
 * a plain Error on network/5xx trouble (keep the session, retry later). */
export async function refreshSession(s: Session): Promise<Session> {
  const res = await fetch(`${SUPABASE_URL}/auth/v1/token?grant_type=refresh_token`, {
    method: "POST", headers: headers(), body: JSON.stringify({ refresh_token: s.refreshToken }),
  });
  if (res.status === 400 || res.status === 401) throw new AuthRejected(`auth refresh ${res.status}`);
  if (!res.ok) throw new Error(`auth refresh ${res.status}`);
  const next = await fromTokens((await res.json()) as TokenReply);
  write(STORE_KEY, next);
  return next;
}

/** Seconds until s should be refreshed (0 = now). */
export const refreshIn = (s: Session): number => Math.max(0, s.expiresAt - REFRESH_EARLY_S - nowS());

export const isRejected = (err: unknown): boolean => err instanceof AuthRejected;

/** Boot: provider return (PKCE) > stored session (refreshed if near expiry) > none.
 * A refresh that fails for network reasons keeps the stored session. */
export async function restoreSession(): Promise<Session | null> {
  const fresh = await consumeRedirect();
  const stored = read<unknown>(STORE_KEY);
  let s = fresh ?? (isSession(stored) ? stored : null);
  if (s && refreshIn(s) === 0) {
    try { s = await refreshSession(s); } catch (err) { if (isRejected(err)) s = null; }
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
