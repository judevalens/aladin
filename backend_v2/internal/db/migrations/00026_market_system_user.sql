-- +goose Up
-- Broadcast market events go through the outbox, but outbox_events.user_id is FK-bound to
-- users (and NOT NULL). A quote isn't owned by anyone, so it needs a stable OWNER row that
-- exists purely to satisfy the FK — the drainer ignores it and routes market events
-- tenant-agnostically (broadcast). A fixed sentinel id (repo.marketQuoteUserID). No
-- password_hash ⇒ it can never authenticate.
INSERT INTO public.users (id, email, created_at, updated_at)
VALUES ('00000000-0000-0000-0000-000000000000', 'system+market@aladin.local', now(), now())
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DELETE FROM public.users WHERE id = '00000000-0000-0000-0000-000000000000';
