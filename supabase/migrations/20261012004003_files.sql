insert into storage.buckets (id, name, public) values ('files', 'files', false);

create policy files_owner_select on storage.objects
  for select to authenticated
  using (bucket_id = 'files' and (storage.foldername(name))[1] = (select auth.uid())::text);
