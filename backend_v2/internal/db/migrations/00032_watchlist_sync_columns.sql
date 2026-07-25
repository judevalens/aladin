-- +goose Up
-- Put watchlists on the R1 sync spine. A watchlist is ONE synced entity (its frame data carries the
-- ordered items[]), so seq/is_deleted live on the parent only:
--   seq        — per-list version (staleness guard), bumped on ANY change to the list OR its members
--   is_deleted — tombstone (row KEPT so a stale lower-seq upsert can't resurrect it; reads filter it)
-- Split out from 00031 as a FORWARD migration because 00031 already shipped to live DBs without these
-- columns (goose won't re-run an edited migration). Items carry no seq — an add/remove re-emits the list.
ALTER TABLE public.watchlists ADD COLUMN IF NOT EXISTS seq        bigint  NOT NULL DEFAULT 0;
ALTER TABLE public.watchlists ADD COLUMN IF NOT EXISTS is_deleted boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE public.watchlists DROP COLUMN IF EXISTS is_deleted;
ALTER TABLE public.watchlists DROP COLUMN IF EXISTS seq;
