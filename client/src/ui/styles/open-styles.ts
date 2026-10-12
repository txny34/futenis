import { StylesModal } from "./styles-modal";

/** The dock's "Estilos" action: builds the panel on first use (its preview owns a
 * second WebGL context) and opens it unless `blocked` (another menu is up). */
export function stylesOpener(blocked: () => boolean, onOpenChange: () => void): () => void {
  let modal: StylesModal | null = null;
  return () => {
    if (blocked()) return;
    if (!modal) {
      modal = new StylesModal(document.body);
      modal.onOpenChange(onOpenChange);
    }
    modal.open();
  };
}
