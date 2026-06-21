-- +goose Up
-- Per-claim version for the sync outbox (the staleness guard): claims become a synced entity
-- ("signal" kind) pushed as data_event frames into the client's local store. The client applies
-- a frame iff its seq > the stored seq, so every projection-changing write bumps this.
ALTER TABLE public.claims ADD COLUMN seq bigint NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE public.claims DROP COLUMN seq;
