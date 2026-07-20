-- +goose Up
-- T1 market data, layer 0 (cont.): OHLCV `bars` (TRADING_PRD.md §5 "store raw bars, adjust
-- on read"). Global/shared like `instruments` (a bar isn't owned by a user), keyed on the
-- stable instrument_id. RAW/unadjusted — corporate-action adjustment is computed at read
-- time (a later `corporate_actions` log), so stored history never mutates under a split and
-- backtests stay reproducible.
CREATE TABLE public.bars (
    instrument_id uuid             NOT NULL,
    timeframe     text             NOT NULL,   -- '1Day' (daily first) | '1Hour' | '1Min' | ...
    ts            timestamptz      NOT NULL,   -- bar start, UTC
    open          double precision NOT NULL,
    high          double precision NOT NULL,
    low           double precision NOT NULL,
    close         double precision NOT NULL,
    volume        bigint           NOT NULL DEFAULT 0,
    PRIMARY KEY (instrument_id, timeframe, ts),
    CONSTRAINT bars_instrument_fk FOREIGN KEY (instrument_id)
        REFERENCES public.instruments(instrument_id) ON DELETE CASCADE
);
-- The read pattern: latest N bars for one instrument+timeframe (chart, trailing window).
CREATE INDEX bars_instrument_tf_ts_idx ON public.bars (instrument_id, timeframe, ts DESC);

-- +goose Down
DROP TABLE IF EXISTS public.bars;
