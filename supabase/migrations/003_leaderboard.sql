-- Global leaderboard: public name and ranked games/wins next to the ELO.
-- Still written only by the game server: the name comes from the verified
-- login (filtered for slurs on the server), games/wins are counted
-- atomically by ranked_result (callable only with the secret key).
alter table public.ratings
  add column if not exists name      text    not null default '' check (char_length(name) <= 24),
  add column if not exists games_1v1 integer not null default 0 check (games_1v1 >= 0),
  add column if not exists games_2v2 integer not null default 0 check (games_2v2 >= 0),
  add column if not exists wins_1v1  integer not null default 0 check (wins_1v1 >= 0),
  add column if not exists wins_2v2  integer not null default 0 check (wins_2v2 >= 0);

alter table public.ratings
  add column if not exists wins integer generated always as (wins_1v1 + wins_2v2) stored;

create index if not exists ratings_top_1v1 on public.ratings (elo_1v1 desc) where games_1v1 > 0;
create index if not exists ratings_top_2v2 on public.ratings (elo_2v2 desc) where games_2v2 > 0;
create index if not exists ratings_top_wins on public.ratings (wins desc) where wins > 0;

-- One ranked result: p_rows = [{user_id, elo, won, name}], p_mode = '1v1' | '2v2'.
create or replace function public.ranked_result(p_mode text, p_rows jsonb) returns void
  language plpgsql security definer set search_path = '' as $$
declare
  r   jsonb;
  one boolean := p_mode = '1v1';
begin
  if p_mode not in ('1v1', '2v2') or jsonb_typeof(p_rows) <> 'array' or jsonb_array_length(p_rows) > 4 then
    raise exception 'ranked_result: bad arguments';
  end if;
  for r in select value from jsonb_array_elements(p_rows) loop
    insert into public.ratings as t
      (user_id, name, elo_1v1, elo_2v2, games_1v1, games_2v2, wins_1v1, wins_2v2, updated_at)
    values (
      (r->>'user_id')::uuid,
      left(coalesce(r->>'name', ''), 24),
      case when one then (r->>'elo')::int else 1000 end,
      case when one then 1000 else (r->>'elo')::int end,
      case when one then 1 else 0 end,
      case when one then 0 else 1 end,
      case when one and (r->>'won')::boolean then 1 else 0 end,
      case when not one and (r->>'won')::boolean then 1 else 0 end,
      now())
    on conflict (user_id) do update set
      name      = case when excluded.name <> '' then excluded.name else t.name end,
      elo_1v1   = case when one then excluded.elo_1v1 else t.elo_1v1 end,
      elo_2v2   = case when one then t.elo_2v2 else excluded.elo_2v2 end,
      games_1v1 = t.games_1v1 + excluded.games_1v1,
      games_2v2 = t.games_2v2 + excluded.games_2v2,
      wins_1v1  = t.wins_1v1 + excluded.wins_1v1,
      wins_2v2  = t.wins_2v2 + excluded.wins_2v2,
      updated_at = now();
  end loop;
end $$;

revoke all on function public.ranked_result(text, jsonb) from public, anon, authenticated;
