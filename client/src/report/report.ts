import { Mode, Phase, SNAP_LEN, Snap } from "../snapshot";
import { recentErrors } from "./errors";

// Plain-text bug report: what the player wrote + enough context to reproduce.
// No personal data: no profile name, no room code (URL query is dropped).
export interface ReportContext {
  note: string;
  /** Latest snapshot (curr), or null before wasm loads. */
  snap: Float64Array | null;
  inLobby: boolean;
  settings: unknown;
}

const MODE_NAME: Readonly<Record<number, string | undefined>> = {
  [Mode.OneVsOne]: "1v1", [Mode.TwoVsTwo]: "2v2", [Mode.Playground]: "entrenamiento", [Mode.Lobby]: "lobby",
};
const PHASE_NAME: Readonly<Record<number, string | undefined>> = {
  [Phase.Serve]: "saque", [Phase.Rally]: "rally", [Phase.Point]: "punto", [Phase.GameOver]: "fin",
};

function gameLines(snap: Float64Array | null, inLobby: boolean): string[] {
  if (!snap) return ["juego: sin cargar"];
  const mode = snap[Snap.Mode];
  return [
    `modo: ${MODE_NAME[mode] ?? mode}${inLobby ? " (lobby)" : ""}`,
    `fase: ${PHASE_NAME[snap[Snap.Phase]] ?? snap[Snap.Phase]} · tick ${snap[Snap.Tick]}`,
    `marcador: ${snap[Snap.Score0]}-${snap[Snap.Score1]} · snapLen ${SNAP_LEN}`,
  ];
}

export function buildReport(c: Readonly<ReportContext>): string {
  const errors = recentErrors();
  return [
    "# Reporte de bug — futenis",
    "",
    c.note.trim() || "(sin descripción)",
    "",
    "## Contexto",
    `fecha: ${new Date().toISOString()}`,
    `página: ${location.origin}${location.pathname}`,
    `navegador: ${navigator.userAgent}`,
    `pantalla: ${screen.width}x${screen.height} @${devicePixelRatio} · ventana ${innerWidth}x${innerHeight}`,
    ...gameLines(c.snap, c.inLobby),
    "",
    "## Ajustes",
    JSON.stringify(c.settings),
    "",
    `## Errores recientes (${errors.length})`,
    ...(errors.length ? errors : ["ninguno"]),
  ].join("\n");
}
