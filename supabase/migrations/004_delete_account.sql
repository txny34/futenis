-- Self-service account deletion (legal requirement). The signed-in player calls
-- /rest/v1/rpc/delete_my_account with their own token; it removes their auth
-- user, and ratings + profiles go with it (on delete cascade, 001 and 002).
create or replace function public.delete_my_account() returns void
  language plpgsql security definer set search_path = '' as $$
declare
  me uuid := auth.uid();
begin
  if me is null then
    raise exception 'delete_my_account: not signed in';
  end if;
  delete from auth.users where id = me;
end;
$$;

revoke all on function public.delete_my_account() from public, anon;
grant execute on function public.delete_my_account() to authenticated;

-- A deleted player's token stays valid until it expires: skip rows whose
-- account is gone so the opponent's result still saves (same body as 003).
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
    if (r->>'elo') is null or (r->>'elo')::int not between 0 and 5000 then
      raise exception 'ranked_result: bad elo';
    end if;
    continue when not exists (select 1 from auth.users u where u.id = (r->>'user_id')::uuid);
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
