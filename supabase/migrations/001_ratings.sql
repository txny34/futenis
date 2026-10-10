-- Ranked ELO per account. Written only by the game server (secret key, bypasses
-- RLS); readable by anyone (leaderboards). No insert/update/delete policies:
-- browsers can never change a rating.
create table if not exists public.ratings (
  user_id    uuid primary key references auth.users (id) on delete cascade,
  elo_1v1    integer not null default 1000 check (elo_1v1 between 0 and 5000),
  elo_2v2    integer not null default 1000 check (elo_2v2 between 0 and 5000),
  updated_at timestamptz not null default now()
);

alter table public.ratings enable row level security;

drop policy if exists "ratings are public" on public.ratings;
create policy "ratings are public" on public.ratings for select using (true);
