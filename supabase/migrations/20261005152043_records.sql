create table public.records (
  id uuid not null,
  user_id uuid not null references auth.users (id) on delete cascade,
  note varchar(65536) not null default '',
  secret varchar(65536) not null,
  updated_at timestamptz not null default now(),
  primary key (user_id, id)
);

alter table public.records enable row level security;

revoke all on public.records from anon, authenticated;
grant select on public.records to authenticated;

create policy records_owner_select on public.records
  for select to authenticated using ((select auth.uid()) = user_id);
