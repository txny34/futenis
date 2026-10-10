// Base URL of the online server. Deployed builds inject FUTENIS_SERVER through
// cmd/dev's esbuild Define; local dev falls back to the same host on :8091.
declare const __FUTENIS_SERVER__: string;
const DEV_PORT = 8091;

export function serverBase(): string {
  if (__FUTENIS_SERVER__) return __FUTENIS_SERVER__.replace(/\/+$/, "");
  return `${location.protocol === "https:" ? "wss" : "ws"}://${location.hostname}:${DEV_PORT}`;
}
