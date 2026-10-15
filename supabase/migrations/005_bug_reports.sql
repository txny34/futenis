-- In-game bug reports ("Reportar un bug" > Enviar). Players (guests too) can
-- only insert; nobody reads them through the API: the team reads them in the
-- dashboard. user_id is the signed-in account (null for guests) and is set by
-- the database, never trusted from the client.
create table if not exists public.bug_reports (
  id         bigint generated always as identity primary key,
  created_at timestamptz not null default now(),
  user_id    uuid default auth.uid() references auth.users (id) on delete set null,
  body       text not null check (char_length(body) between 1 and 12000),
  client     text not null default '' -- md5 of the account id or the IP (set by the trigger, for rate limits)
);

create index if not exists bug_reports_client_recent on public.bug_reports (client, created_at desc);

alter table public.bug_reports enable row level security;

drop policy if exists bug_reports_insert on public.bug_reports;
create policy bug_reports_insert on public.bug_reports
  for insert to anon, authenticated
  with check (user_id is not distinct from auth.uid());

revoke all on public.bug_reports from anon, authenticated;
grant insert (body) on public.bug_reports to anon, authenticated;

-- Spam guard: 5 reports per account or IP per 10 minutes, 300 per hour overall.
-- cf-connecting-ip is set by Supabase's edge (not spoofable by the browser).
create or replace function public.bug_reports_limit() returns trigger
  language plpgsql security definer set search_path = '' as $$
declare
  h   json := coalesce(nullif(current_setting('request.headers', true), '')::json, '{}'::json);
  who text := coalesce(auth.uid()::text, h->>'cf-connecting-ip', h->>'x-real-ip', split_part(coalesce(h->>'x-forwarded-for', ''), ',', 1));
begin
  new.client := md5(coalesce(who, ''));
  if (select count(*) from public.bug_reports
      where client = new.client and created_at > now() - interval '10 minutes') >= 5
     or (select count(*) from public.bug_reports where created_at > now() - interval '1 hour') >= 300 then
    raise exception 'bug_reports: too many reports, try later';
  end if;
  return new;
end $$;

revoke all on function public.bug_reports_limit() from public, anon, authenticated;
drop trigger if exists bug_reports_limit on public.bug_reports;
create trigger bug_reports_limit before insert on public.bug_reports
  for each row execute function public.bug_reports_limit();
