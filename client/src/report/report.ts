import { t } from "../i18n/t";
import { Mode, Phase, SNAP_LEN, Snap } from "../snapshot";
import { recentErrors } from "./errors";

// Plain-text bug report: what the player wrote + enough context to reproduce.
// No profile name or room code (URL query is dropped); a sent report is
// linked to the signed-in account by the database.
export interface ReportContext {
  note: string;
  /** Latest snapshot (curr), or null before wasm loads. */
  snap: Float64Array | null;
  inLobby: boolean;
  settings: unknown;
}

const MODE_NAME: Readonly<Record<number, string | undefined>> = {
  [Mode.OneVsOne]: "1v1", [Mode.TwoVsTwo]: "2v2", [Mode.Playground]: t("entrenamiento", "training"), [Mode.Lobby]: "lobby",
};
const PHASE_NAME: Readonly<Record<number, string | undefined>> = {
  [Phase.Serve]: t("saque", "serve"), [Phase.Rally]: "rally", [Phase.Point]: t("punto", "point"), [Phase.GameOver]: t("fin", "end"),
};

function gameLines(snap: Float64Array | null, inLobby: boolean): string[] {
  if (!snap) return [t("juego: sin cargar", "game: not loaded")];
  const mode = snap[Snap.Mode];
  return [
    `${t("modo", "mode")}: ${MODE_NAME[mode] ?? mode}${inLobby ? " (lobby)" : ""}`,
    `${t("fase", "phase")}: ${PHASE_NAME[snap[Snap.Phase]] ?? snap[Snap.Phase]} · tick ${snap[Snap.Tick]}`,
    `${t("marcador", "score")}: ${snap[Snap.Score0]}-${snap[Snap.Score1]} · snapLen ${SNAP_LEN}`,
  ];
}

export function buildReport(c: Readonly<ReportContext>): string {
  const errors = recentErrors();
  return [
    t("# Reporte de bug — futenis", "# Bug report — futenis"),
    "",
    c.note.trim() || t("(sin descripción)", "(no description)"),
    "",
    t("## Contexto", "## Context"),
    `${t("fecha", "date")}: ${new Date().toISOString()}`,
    `${t("página", "page")}: ${location.origin}${location.pathname}`,
    `${t("navegador", "browser")}: ${navigator.userAgent}`,
    `${t("pantalla", "screen")}: ${screen.width}x${screen.height} @${devicePixelRatio} · ${t("ventana", "window")} ${innerWidth}x${innerHeight}`,
    ...gameLines(c.snap, c.inLobby),
    "",
    t("## Ajustes", "## Settings"),
    JSON.stringify(c.settings),
    "",
    `${t("## Errores recientes", "## Recent errors")} (${errors.length})`,
    ...(errors.length ? errors : [t("ninguno", "none")]),
  ].join("\n");
}
