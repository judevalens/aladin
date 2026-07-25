-- +goose Up
-- Named / multiple watchlists, modeled as a UNIVERSE primitive (not a UI-only list): a named set
-- of instruments the trading engine consumes/produces. A `watchlists` parent owns many
-- watchlist_items (the `manual` members); `kind` + `definition` reserve the dynamic side — a
-- `screener` resolves its set from a stored rule, `hybrid` = manual picks ∪ rule. The engine's one
-- port is "resolve this watchlist to instruments"; T2 strategies use a watchlist_id as their
-- universe, T3 scans populate watchlists, alerts can target one. v1 builds `manual`; screener
-- evaluation is a fast-follow (definition is stored but not yet evaluated).
--
-- The same instrument may live in several lists (new PK is (watchlist_id, instrument_id)). Existing
-- flat rows migrate into a per-user default list so nothing is lost. user_id stays on
-- watchlist_items (denormalized) so ownership is a WHERE clause with no join on the hot path.
CREATE TABLE public.watchlists (
    id         uuid PRIMARY KEY,
    user_id    uuid NOT NULL,
    name       text NOT NULL DEFAULT 'Watchlist',
    -- kind determines how membership resolves: explicit rows (manual), a stored rule (screener),
    -- or both (hybrid). definition holds the screener rule (empty for manual).
    kind       text NOT NULL DEFAULT 'manual',
    definition jsonb NOT NULL DEFAULT '{}'::jsonb,
    position   bigint NOT NULL DEFAULT 0,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT watchlists_kind_chk CHECK (kind IN ('manual','screener','hybrid'))
);
CREATE INDEX watchlists_user_idx ON public.watchlists (user_id, position);

-- Backfill: one default (manual) list per DISTINCT user that already has flat watchlist items.
INSERT INTO public.watchlists (id, user_id, name, kind, position)
SELECT gen_random_uuid(), u.user_id, 'Watchlist', 'manual', 0
  FROM (SELECT DISTINCT user_id FROM public.watchlist_items) u;

-- Grow watchlist_items: attach to a list + carry an ordering column (v1 orders by added_at; the
-- position column ships for a future drag-reorder).
ALTER TABLE public.watchlist_items ADD COLUMN watchlist_id uuid;
ALTER TABLE public.watchlist_items ADD COLUMN position bigint NOT NULL DEFAULT 0;

-- Point every existing item at its owner's freshly-created default list.
UPDATE public.watchlist_items wi
   SET watchlist_id = wl.id
  FROM public.watchlists wl
 WHERE wl.user_id = wi.user_id;

-- Enforce presence + swap the PK from (user_id, instrument_id) to (watchlist_id, instrument_id).
ALTER TABLE public.watchlist_items ALTER COLUMN watchlist_id SET NOT NULL;
ALTER TABLE public.watchlist_items DROP CONSTRAINT watchlist_items_pkey;
ALTER TABLE public.watchlist_items ADD PRIMARY KEY (watchlist_id, instrument_id);
ALTER TABLE public.watchlist_items
    ADD CONSTRAINT watchlist_items_list_fk FOREIGN KEY (watchlist_id)
        REFERENCES public.watchlists(id) ON DELETE CASCADE;
CREATE INDEX watchlist_items_list_idx ON public.watchlist_items (watchlist_id, position);

-- +goose Down
-- Best-effort flatten back to the per-(user, instrument) shape. TRADEOFF: the same instrument in
-- >1 list cannot fit the old (user_id, instrument_id) PK, so duplicates collapse to one row per
-- user+instrument (the instrument stays on the user's watchlist; only the multi-list membership is
-- lost). Acceptable for a rollback path only.
ALTER TABLE public.watchlist_items DROP CONSTRAINT watchlist_items_list_fk;
ALTER TABLE public.watchlist_items DROP CONSTRAINT watchlist_items_pkey;
DROP INDEX IF EXISTS public.watchlist_items_list_idx;

DELETE FROM public.watchlist_items a
 USING public.watchlist_items b
 WHERE a.ctid > b.ctid
   AND a.user_id = b.user_id
   AND a.instrument_id = b.instrument_id;

ALTER TABLE public.watchlist_items DROP COLUMN watchlist_id;
ALTER TABLE public.watchlist_items DROP COLUMN position;
ALTER TABLE public.watchlist_items ADD PRIMARY KEY (user_id, instrument_id);

DROP TABLE IF EXISTS public.watchlists;
