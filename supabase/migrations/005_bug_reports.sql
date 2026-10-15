-- In-game bug reports ("Reportar un bug" > Enviar). Players (guests too) can
-- only insert; nobody reads them through the API: the team reads them in the
-- dashboard. user_id is the signed-in account (null for guests) and is set by
-- the database, never trusted from the client.
create table if not exists public.bug_reports (
  id         bigint generated always as identity primary key,
  created_at timestamptz not null default now(),
  user_id    uuid default auth.uid() references auth.users (id) on delete set null,
  body       text not null check (char_length(body) between 1 and 12000)
);

alter table public.bug_reports enable row level security;

drop policy if exists bug_reports_insert on public.bug_reports;
create policy bug_reports_insert on public.bug_reports
  for insert to anon, authenticated
  with check (user_id is not distinct from auth.uid());

revoke all on public.bug_reports from anon, authenticated;
grant insert (body) on public.bug_reports to anon, authenticated;
