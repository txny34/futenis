import { t } from "../i18n/t";

// The stored default name stays "Invitado" (language-neutral data); only the
// default is localized when shown. A name the player chose is shown as is.
const DEFAULT_NAME = "Invitado"; // mirrors profile/store.ts default

export function displayName(stored: string): string {
  return stored === DEFAULT_NAME ? t("Invitado", "Guest") : stored;
}
