import { t } from "../i18n/t";
import { button, el } from "./dom";
import { panel } from "./screens";

// "Borrar cuenta": one explicit confirm button (no browser dialog), a status
// line while it runs, and the error text if the server refuses.
export function deleteAccountView(run: () => Promise<void>, done: () => void, back: () => void): HTMLElement {
  const status = el("p", "fx-report__hint", "", { role: "status" });
  const confirm = button(t("Borrar mi cuenta para siempre", "Delete my account forever"), () => {
    confirm.disabled = true;
    status.textContent = t("Borrando…", "Deleting…");
    run().then(done, () => {
      confirm.disabled = false;
      status.textContent = t("No se pudo borrar la cuenta. Probá de nuevo en un rato.", "Could not delete the account. Please try again later.");
    });
  }, "fx-btn fx-btn--danger");
  const warning = el("p", "fx-report__hint", t(
    "Se borran tu cuenta, tu ELO, tu lugar en el ranking y tu progreso en la nube. No se puede deshacer. El progreso guardado en este navegador queda como invitado.",
    "Your account, ELO, leaderboard spot and cloud progress are deleted. This cannot be undone. Progress saved in this browser stays as a guest.",
  ));
  return panel(t("Borrar cuenta", "Delete account"), warning, confirm, status, button(t("Volver", "Back"), back, "fx-btn fx-btn--ghost"));
}
