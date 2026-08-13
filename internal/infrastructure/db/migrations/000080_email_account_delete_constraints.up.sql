-- A mailbox can have queued send/warmup work and admin audit rows. These are
-- operational records, not campaign history, and must not prevent a user from
-- disconnecting the mailbox. Campaign analytics remain in their own tables.
ALTER TABLE public.tasks
    DROP CONSTRAINT IF EXISTS tasks_email_account_id_fkey;

ALTER TABLE public.tasks
    ADD CONSTRAINT tasks_email_account_id_fkey
    FOREIGN KEY (email_account_id)
    REFERENCES public.email_accounts(id)
    ON DELETE CASCADE;

ALTER TABLE public.warmup_admin_actions
    DROP CONSTRAINT IF EXISTS warmup_admin_actions_email_account_id_fkey;

ALTER TABLE public.warmup_admin_actions
    ADD CONSTRAINT warmup_admin_actions_email_account_id_fkey
    FOREIGN KEY (email_account_id)
    REFERENCES public.email_accounts(id)
    ON DELETE CASCADE;
