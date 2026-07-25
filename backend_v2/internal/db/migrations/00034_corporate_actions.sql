-- +goose Up
-- T1 / L0 — the corporate-actions log that makes "store raw bars, adjust on read" possible
-- (TRADING_PRD §5). Adjusted prices mutate retroactively on every split and dividend, so storing
-- them would make stored history silently change and backtests non-reproducible. We store
-- UNADJUSTED bars plus this log and compute the adjustment at read time: deterministic, replayable,
-- and the adjustment logic itself is unit-testable.
--
-- Keyed by instrument_id, never symbol (§5: tickers get recycled; instrument_id is the stable
-- identity). ex_date is the first session on which the price trades WITHOUT the entitlement — bars
-- STRICTLY BEFORE it are the ones that need adjusting.
CREATE TABLE public.corporate_actions (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    instrument_id uuid NOT NULL,
    type          text NOT NULL,
    ex_date       date NOT NULL,
    -- split only: NEW shares per OLD share (a 4-for-1 split is 4.0; a 1-for-10 reverse is 0.1).
    -- Pre-split prices divide by this, pre-split volume multiplies by it.
    split_ratio      double precision,
    -- cash_dividend only: cash per share, instrument currency.
    cash_amount      double precision,
    source        text NOT NULL DEFAULT 'alpaca',
    created_at    timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT corporate_actions_type_chk CHECK (type IN ('split', 'cash_dividend')),
    -- Each type carries exactly the field it needs — a split without a ratio (or a dividend without
    -- an amount) is unusable at read time, so reject it at write time.
    CONSTRAINT corporate_actions_payload_chk CHECK (
        (type = 'split'         AND split_ratio IS NOT NULL AND split_ratio > 0 AND cash_amount IS NULL) OR
        (type = 'cash_dividend' AND cash_amount IS NOT NULL AND cash_amount > 0 AND split_ratio IS NULL)
    ),
    -- One action of a kind per instrument per ex-date → vendor re-syncs upsert instead of duplicating
    -- (a duplicated split would double-adjust all prior history).
    CONSTRAINT corporate_actions_unique UNIQUE (instrument_id, type, ex_date),
    CONSTRAINT corporate_actions_instrument_fk FOREIGN KEY (instrument_id)
        REFERENCES public.instruments(instrument_id) ON DELETE CASCADE
);

-- The read pattern is "all actions for this instrument, in ex_date order" (the adjustment walk).
CREATE INDEX corporate_actions_instrument_ex ON public.corporate_actions (instrument_id, ex_date);

-- +goose Down
DROP TABLE IF EXISTS public.corporate_actions;
