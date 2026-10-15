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
