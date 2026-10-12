import { authUser, onAuthChange } from "../auth/auth";
import { profileStore } from "../profile/store";

// The name printed on my own shirt: the account name when signed in, else
// the name I chose; a guest's default name prints nothing (number only).

const GUEST_NAME = "Invitado"; // mirrors profile/store.ts default

export function myShirtName(): string {
  const account = authUser()?.name;
  if (account) return account;
  const stored = profileStore().get().name;
  return stored === GUEST_NAME ? "" : stored;
}

/** Calls f whenever my shirt name may have changed (sign in/out, rename). */
export function onShirtNameChange(f: () => void): void {
  onAuthChange(f);
  profileStore().onChange(f);
}
