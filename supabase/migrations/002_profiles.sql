-- Cloud progress per account (level, XP, missions, history, medals). Written by
-- the player's own browser, so it is cosmetic: ranked ELO stays in
-- public.ratings, written only by the game server. Each account reads and
-- writes only its own row; the blob is capped so nobody can park big data here.
create table if not exists public.profiles (
  user_id    uuid primary key references auth.users (id) on delete cascade,
  data       jsonb not null default '{}'::jsonb
             check (jsonb_typeof(data) = 'object' and octet_length(data::text) <= 16384),
  updated_at timestamptz not null default now()
);

alter table public.profiles enable row level security;

drop policy if exists "own profile read" on public.profiles;
create policy "own profile read" on public.profiles for select
  using ((select auth.uid()) = user_id);

drop policy if exists "own profile insert" on public.profiles;
create policy "own profile insert" on public.profiles for insert
  with check ((select auth.uid()) = user_id);

drop policy if exists "own profile update" on public.profiles;
create policy "own profile update" on public.profiles for update
  using ((select auth.uid()) = user_id) with check ((select auth.uid()) = user_id);

create or replace function public.profiles_touch() returns trigger
  language plpgsql set search_path = '' as $$
begin
  new.updated_at := now();
  return new;
end $$;

drop trigger if exists profiles_touch on public.profiles;
create trigger profiles_touch before update on public.profiles
  for each row execute function public.profiles_touch();
