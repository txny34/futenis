import { isRejected, refreshIn, refreshSession, restoreSession, signOut, type AuthUser, type Session } from "./session";

// Who is playing: a signed-in account or a guest. One session per page, kept
// fresh by a timer; UI subscribes with onAuthChange. The account name is shown
// instead of the profile name (profile/display-name.ts) without overwriting it.
const RETRY_S = 30;
let current: Session | null = null;
let timer = 0;
const listeners: Array<(u: AuthUser | null) => void> = [];

export const authUser = (): AuthUser | null => current?.user ?? null;
export const accessToken = (): string | null => current?.accessToken ?? null;

export function onAuthChange(cb: (u: AuthUser | null) => void): void { listeners.push(cb); }

function set(s: Session | null): void {
  const changed = (s?.user.id ?? null) !== (current?.user.id ?? null);
  current = s;
  window.clearTimeout(timer);
  if (s) timer = window.setTimeout(() => { void renew(); }, refreshIn(s) * 1000);
  if (changed) for (const cb of listeners) cb(s?.user ?? null);
}

async function renew(): Promise<void> {
  const s = current;
  if (!s) return;
  try {
    set(await refreshSession(s));
  } catch (err) {
    if (isRejected(err)) { await logout(); return; }
    timer = window.setTimeout(() => { void renew(); }, RETRY_S * 1000); // offline: try again
  }
}

/** Boot: picks up a provider return or the stored session. Never throws. */
export async function initAuth(): Promise<string | null> {
  try {
    set(await restoreSession());
    return null;
  } catch (err) {
    set(null);
    return err instanceof Error ? err.message : String(err);
  }
}

export async function logout(): Promise<void> {
  const s = current;
  set(null);
  await signOut(s);
}
