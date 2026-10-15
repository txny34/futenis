// Ring of the last uncaught errors / rejections, attached to bug reports.
// Sent only inside a bug report the player chooses to send.
const MAX_ERRORS = 20;
const MAX_LEN = 300;

const QUERY = /[?#][^\s'"]*/g; // URL queries and fragments can carry room codes

const log: string[] = [];
let installed = false;

function push(msg: string): void {
  log.push(`${new Date().toISOString()} ${msg.replace(QUERY, "?…").slice(0, MAX_LEN)}`);
  if (log.length > MAX_ERRORS) log.shift();
}

/** Call once at boot. */
export function captureErrors(): void {
  if (installed) return;
  installed = true;
  window.addEventListener("error", (e) => push(`${e.message} @ ${e.filename}:${e.lineno}`));
  window.addEventListener("unhandledrejection", (e) => {
    const r: unknown = e.reason;
    push(`unhandled: ${r instanceof Error ? r.message : String(r)}`);
  });
}

export function recentErrors(): readonly string[] { return [...log]; }
