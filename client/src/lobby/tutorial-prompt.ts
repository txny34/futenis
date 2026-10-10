import { t } from "../i18n/t";
import { tutorialDone } from "../tutorial/tutorial";
import { button, el } from "../ui/dom";

// Small top-left card in the lobby: "¿Primera vez?" -> tutorial, never forced.
// Hidden for good once the tutorial is done or the player closes it. Lives inside
// the lobby bar's root, so it shows and hides with the lobby.
const DISMISS_KEY = "futenis.tutorial.prompt.closed";

function dismissed(): boolean {
  try { return localStorage.getItem(DISMISS_KEY) === "1"; } catch { return false; }
}

export class TutorialPrompt {
  private readonly root = el("aside", "fx-tutprompt", "", { "aria-label": t("Tutorial", "Tutorial") });

  constructor(parent: HTMLElement, onStart: () => void) {
    const close = button("✕", () => this.dismiss(), "fx-tutprompt__close");
    close.setAttribute("aria-label", t("No, gracias", "No thanks"));
    this.root.append(close, el("b", "fx-tutprompt__title", t("¿Primera vez?", "First time?")),
      el("p", "fx-tutprompt__text", t("Aprendé a jugar en 2 minutos.", "Learn to play in 2 minutes.")),
      button(t("Hacer el tutorial", "Play the tutorial"), () => { this.dismiss(); onStart(); }, "fx-btn fx-tutprompt__go"));
    this.root.hidden = tutorialDone() || dismissed();
    parent.append(this.root);
  }

  private dismiss(): void {
    this.root.hidden = true;
    try { localStorage.setItem(DISMISS_KEY, "1"); } catch { /* blocked storage: offered again next visit */ }
  }
}
