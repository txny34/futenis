import { accessToken, authUser, onAuthChange } from "../auth/auth";
import { SUPABASE_KEY, SUPABASE_URL } from "../auth/config";
import { asSnapshot, mergeSnapshots, PARTS, type Part, type Snapshot } from "./cloud-merge";
import { missions } from "./missions";
import { profileWriter } from "./store";

// Cloud progress (table public.profiles, own row only, RLS). Signing in pulls the
// account's copy and merges it with this browser's (guest progress is adopted);
// changes are pushed a moment later and when the tab is hidden. A browser whose
// progress belongs to another account takes the cloud copy instead of merging.
const OWNER_KEY = "futenis.cloud.owner";
const PUSH_DELAY_MS = 3000;
const PULL_EVERY_MS = 30_000;
const UUID = /^[0-9a-f-]{36}$/;

let synced: string | null = null; // account whose cloud copy is merged in (pushes allowed)
let timer = 0;
let lastPull = 0;
let chain: Promise<void> = Promise.resolve(); // one sync at a time: no out-of-order pushes

const PENDING_PREFIX = "futenis.cloud.pending.";
function readLocal(): Snapshot {
  const out: Snapshot = {};
  for (const p of Object.keys(PARTS) as Part[]) {
    try {
      const raw = localStorage.getItem(PARTS[p]);
      if (raw !== null) out[p] = JSON.parse(raw);
    } catch { /* corrupt or blocked: that part starts empty */ }
  }
  return out;
}

function writeLocal(s: Snapshot): void {
  try {
    for (const p of Object.keys(PARTS) as Part[]) {
      if (s[p] === undefined) localStorage.removeItem(PARTS[p]);
      else localStorage.setItem(PARTS[p], JSON.stringify(s[p]));
    }
  } catch { /* storage blocked: progress lives in memory this visit */ }
}

function owner(): string | null {
  try { return localStorage.getItem(OWNER_KEY); } catch { return null; }
}

function setOwner(id: string): void {
  try { localStorage.setItem(OWNER_KEY, id); } catch { /* blocked: merges again next visit */ }
}

function headers(token: string): Record<string, string> {
  return { apikey: SUPABASE_KEY, Authorization: `Bearer ${token}`, "Content-Type": "application/json" };
}

async function fetchCloud(id: string, token: string): Promise<Snapshot | null> {
  const res = await fetch(`${SUPABASE_URL}/rest/v1/profiles?select=data&user_id=eq.${id}`, { headers: headers(token) });
  if (!res.ok) throw new Error(`cloud profile ${res.status}`);
  const rows: unknown = await res.json();
  const row = Array.isArray(rows) ? (rows[0] as { data?: unknown } | undefined) : undefined;
  return row ? asSnapshot(row.data) : null;
}

function stash(id: string, local: Snapshot): void {
  try { localStorage.setItem(PENDING_PREFIX + id, JSON.stringify(local)); } catch { /* blocked: nothing to keep */ }
}

function takeStash(id: string): Snapshot {
  try {
    const raw = localStorage.getItem(PENDING_PREFIX + id);
    localStorage.removeItem(PENDING_PREFIX + id);
    return raw === null ? {} : asSnapshot(JSON.parse(raw));
  } catch { return {}; }
}

/** This browser's copy for account id. Another account's progress is set aside
 * (merged back when that account signs in here again), never adopted or lost. */
function localFor(id: string): Snapshot {
  const prev = owner();
  if (prev !== null && prev !== id && UUID.test(prev)) { stash(prev, readLocal()); return takeStash(id); }
  return mergeSnapshots(readLocal(), takeStash(id));
}

async function upload(id: string, token: string, data: Snapshot): Promise<void> {
  const res = await fetch(`${SUPABASE_URL}/rest/v1/profiles?on_conflict=user_id`, {
    method: "POST",
    headers: { ...headers(token), Prefer: "resolution=merge-duplicates,return=minimal" },
    body: JSON.stringify({ user_id: id, data }),
  });
  if (!res.ok) throw new Error(`cloud push ${res.status}`);
}

/** Pull, merge with this browser, reload the live stores, push the merge. Pushing
 * only merged data means a device that is behind never overwrites the cloud. */
async function sync(id: string): Promise<void> {
  const token = accessToken();
  if (!token || !UUID.test(id)) return;
  lastPull = Date.now();
  const cloud = await fetchCloud(id, token);
  if (authUser()?.id !== id) return; // signed out (or switched) meanwhile
  const local = localFor(id);
  const merged = cloud ? mergeSnapshots(local, cloud) : local;
  writeLocal(merged);
  setOwner(id);
  synced = id;
  profileWriter().reload(); // fires schedule(); the timer is cleared below
  missions().reload();
  window.clearTimeout(timer);
  await upload(id, token, merged);
}

function queueSync(id: string): void {
  chain = chain.then(() => sync(id)).catch(() => {
    window.clearTimeout(timer); // offline or refused: local stays the truth, retried on the next change
  });
}

function schedule(): void {
  if (!synced) return;
  const id = synced;
  window.clearTimeout(timer);
  timer = window.setTimeout(() => queueSync(id), PUSH_DELAY_MS);
}

function onVisibility(): void {
  const u = authUser();
  if (!u) return;
  if (document.visibilityState === "hidden") { window.clearTimeout(timer); if (synced === u.id) queueSync(u.id); return; }
  if (Date.now() - lastPull > PULL_EVERY_MS) queueSync(u.id);
}

/** Wires the sync once at boot (before initAuth resolves). Unpushed changes
 * survive sign-out: the browser keeps them under the owner and the next sign-in
 * of that account merges them. */
export function startCloudSync(): void {
  profileWriter().onChange(schedule);
  missions().onChange(schedule);
  document.addEventListener("visibilitychange", onVisibility);
  onAuthChange((u) => {
    synced = null;
    window.clearTimeout(timer);
    if (u) queueSync(u.id);
  });
}
