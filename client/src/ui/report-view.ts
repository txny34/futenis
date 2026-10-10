import { button, el } from "./dom";
import { panel } from "./screens";

// "Reportar un bug": the player describes it, then copies or downloads the
// report (no backend yet, so nothing is sent from here).
const FILE_NAME = "futenis-bug.txt";
const STATUS_MS = 4000;
const REVOKE_MS = 10000;
const MAX_NOTE = 2000;

export function reportView(build: (note: string) => string, back: () => void): HTMLElement {
  const note = el("textarea", "fx-report__note", "", {
    rows: "5", maxlength: String(MAX_NOTE), "aria-label": "Qué pasó",
    placeholder: "Qué pasó, qué esperabas y cómo repetirlo",
  });
  const status = el("p", "fx-report__status", "", { role: "status" });
  const copy = button("Copiar reporte", () => copyReport(build(note.value), status), "fx-btn fx-btn--hero");
  const save = button("Descargar .txt", () => download(build(note.value)));
  const actions = el("div", "fx-stack");
  actions.append(copy, save, button("Volver", back, "fx-btn fx-btn--ghost"));
  const hint = el("p", "fx-report__hint", "Incluye navegador, modo, ajustes y errores recientes. Sin datos personales.");
  return panel("Reportar un bug", note, hint, status, actions);
}

function copyReport(text: string, status: HTMLElement): void {
  const fail = (): void => flash(status, "No se pudo copiar: usá Descargar.");
  // navigator.clipboard is undefined outside secure contexts (plain http on a LAN IP)
  const clip = (navigator as { clipboard?: Clipboard }).clipboard;
  if (!clip) { fail(); return; }
  clip.writeText(text).then(() => flash(status, "Copiado. Pegalo donde nos reportes."), fail);
}

function flash(status: HTMLElement, text: string): void {
  status.textContent = text;
  window.setTimeout(() => { if (status.textContent === text) status.textContent = ""; }, STATUS_MS);
}

function download(text: string): void {
  const url = URL.createObjectURL(new Blob([text], { type: "text/plain;charset=utf-8" }));
  const a = el("a", "", "", { href: url, download: FILE_NAME });
  document.body.append(a);
  a.click();
  a.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), REVOKE_MS); // a slow save dialog still reads it // revoking in the same task can cancel it
}
