-- +goose Up
-- Worker heartbeat. The worker owns Redis/Asynq; the api is Redis-free (can't inspect the queue),
-- so the worker periodically writes real queue counts + a liveness timestamp here. The api's
-- WorkerStatus reads this row — replacing fabricated zeros with true counts, and detecting a
-- down/wedged worker via a stale updated_at. Single row (id=1).
CREATE TABLE public.worker_heartbeat (
    id         smallint PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    stats      jsonb NOT NULL DEFAULT '{}'::jsonb
);
-- Seed with a stale timestamp so status reads "worker down" until the first real beat.
INSERT INTO public.worker_heartbeat (id, updated_at) VALUES (1, now() - interval '1 hour')
    ON CONFLICT (id) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS public.worker_heartbeat;
