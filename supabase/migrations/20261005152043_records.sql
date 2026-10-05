create table public.device_keys (
  public_key text primary key,
  user_id uuid not null default auth.uid() references auth.users (id) on delete cascade,
  created_at timestamptz not null default now()
);

create table public.records (
  id uuid primary key,
  user_id uuid not null references auth.users (id) on delete cascade,
  note text not null default '',
  secret text not null,
  updated_at timestamptz not null default now()
);

alter table public.device_keys enable row level security;
alter table public.records enable row level security;

revoke all on public.device_keys, public.records from anon, authenticated;
grant insert (public_key) on public.device_keys to authenticated;
grant select on public.records to authenticated;

create policy device_keys_owner_insert on public.device_keys
  for insert to authenticated with check ((select auth.uid()) = user_id);
create policy records_owner_select on public.records
  for select to authenticated using ((select auth.uid()) = user_id);
