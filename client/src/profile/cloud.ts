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
let dirty = false;
let timer = 0;
let lastPull = 0;

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
  const rows = (await res.json()) as Array<{ data?: unknown }>;
  return rows[0] ? asSnapshot(rows[0].data) : null;
}

/** Pulls the account's copy, merges, reloads the live stores and pushes the result. */
async function pull(id: string): Promise<void> {
  const token = accessToken();
  if (!token || !UUID.test(id)) return;
  lastPull = Date.now();
  const cloud = await fetchCloud(id, token);
  if (authUser()?.id !== id) return; // signed out (or switched) meanwhile
  const prev = owner();
  const local = prev !== null && prev !== id ? {} : readLocal(); // another account's progress is not adopted
  writeLocal(cloud ? mergeSnapshots(local, cloud) : local);
  setOwner(id);
  synced = id;
  profileWriter().reload();
  missions().reload();
  dirty = true;
  void push();
}

async function push(keepalive = false): Promise<void> {
  window.clearTimeout(timer);
  const id = synced;
  const token = accessToken();
  if (!dirty || !id || !token || authUser()?.id !== id) return;
  dirty = false;
  try {
    const res = await fetch(`${SUPABASE_URL}/rest/v1/profiles?on_conflict=user_id`, {
      method: "POST", keepalive,
      headers: { ...headers(token), Prefer: "resolution=merge-duplicates,return=minimal" },
      body: JSON.stringify({ user_id: id, data: readLocal() }),
    });
    if (!res.ok) throw new Error(`cloud push ${res.status}`);
  } catch {
    dirty = true; // offline or refused: retried on the next change or tab hide
  }
}

function schedule(): void {
  if (!synced) return;
  dirty = true;
  window.clearTimeout(timer);
  timer = window.setTimeout(() => { void push(); }, PUSH_DELAY_MS);
}

function onVisibility(): void {
  if (document.visibilityState === "hidden") { void push(true); return; }
  const u = authUser();
  if (u && Date.now() - lastPull > PULL_EVERY_MS) void pull(u.id).catch(() => undefined);
}

/** Wires the sync once at boot (before initAuth resolves). */
export function startCloudSync(): void {
  profileWriter().onChange(schedule);
  missions().onChange(schedule);
  document.addEventListener("visibilitychange", onVisibility);
  onAuthChange((u) => {
    synced = null;
    if (u) void pull(u.id).catch(() => undefined); // offline: stays local until the next sign-in or tab return
  });
}
