import { authUser } from "../auth/auth";
import { t } from "../i18n/t";

// The stored default name stays "Invitado" (language-neutral data); only the
// default is localized when shown. A name the player chose is shown as is.
const DEFAULT_NAME = "Invitado"; // mirrors profile/store.ts default

/** Signed in: the account name (the stored guest name is kept for later). */
export function displayName(stored: string): string {
  const account = authUser()?.name;
  if (account) return account;
  return stored === DEFAULT_NAME ? t("Invitado", "Guest") : stored;
}
