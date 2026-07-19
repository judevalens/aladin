-- +goose Up
-- The watchlist — the first trading-platform surface (the "normal trading surface":
-- tickers you're tracking). One flat per-user list for now; named/multiple watchlists are
-- a fast-follow. References the HARD instrument by its stable id, never by symbol.
CREATE TABLE public.watchlist_items (
    user_id       uuid NOT NULL,
    instrument_id uuid NOT NULL,
    added_at      timestamp with time zone DEFAULT now() NOT NULL,
    PRIMARY KEY (user_id, instrument_id),
    CONSTRAINT watchlist_items_instrument_fk FOREIGN KEY (instrument_id)
        REFERENCES public.instruments(instrument_id) ON DELETE CASCADE
);
CREATE INDEX watchlist_items_user_idx ON public.watchlist_items (user_id, added_at DESC);

-- +goose Down
DROP TABLE IF EXISTS public.watchlist_items;
