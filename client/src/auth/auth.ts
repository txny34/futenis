import { profileWriter } from "../profile/store";
import { restoreSession, signOut, type AuthUser, type Session } from "./session";

// Who is playing: a signed-in account or a guest. One session per page; UI
// subscribes with onChange (lobby profile card, login modal, play modal).
let current: Session | null = null;
const listeners: Array<(u: AuthUser | null) => void> = [];

export const authUser = (): AuthUser | null => current?.user ?? null;
export const accessToken = (): string | null => current?.accessToken ?? null;

export function onAuthChange(cb: (u: AuthUser | null) => void): void { listeners.push(cb); }

const GUEST_NAME = "Invitado"; // profile/store.ts default (localized by displayName)

function set(s: Session | null): void {
  const wasSignedIn = current !== null;
  current = s;
  const u = s?.user ?? null;
  if (u?.name) profileWriter().update((p) => ({ ...p, name: u.name })); // the account name is the player name
  else if (!u && wasSignedIn) profileWriter().update((p) => ({ ...p, name: GUEST_NAME }));
  for (const cb of listeners) cb(u);
}

/** Boot: picks up a provider redirect or the stored session. Never throws. */
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
