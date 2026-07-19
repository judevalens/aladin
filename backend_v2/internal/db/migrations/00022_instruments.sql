-- +goose Up
-- T1 market data, layer 0: the `instruments` registry (TRADING_PRD.md §5).
--
-- A security (NVDA), NOT a company — issued_by a company, many securities → one company
-- (GOOG/GOOGL → Alphabet). This is the HARD, externally-keyed trading substrate: the
-- trading spine (bars, signals, trades, backtests) will FK to `instrument_id`, never to
-- the soft entity layer. See project_trading_entity_data_model in memory.
--
-- Identity rule (PRD §5): `instrument_id` is stable; `symbol` is a time-scoped attribute
-- of it (tickers get recycled). Delisted rows are RETAINED with validity dates so the
-- universe can be resolved as-of a date (survivorship-bias-free), never as-now.
--
-- `entity_id` is the nullable bridge to the research/entity layer — the ONE piece of the
-- deferred entity model we land now, so linking a ticker to its research twin is a join,
-- not a retrofit. `cik`/`cusip` are the authoritative external ids the entity resolver's
-- deterministic tier will match on later. Both stay null until that work happens.

CREATE TABLE public.instruments (
    instrument_id uuid        DEFAULT gen_random_uuid() NOT NULL,
    symbol        text        NOT NULL,
    name          text        NOT NULL DEFAULT '',
    exchange      text        NOT NULL DEFAULT '',
    asset_class   text        NOT NULL DEFAULT 'us_equity',
    cusip         text,
    cik           text,
    -- Soft bridge to the entity layer (research twin). Nullable: most instruments never
    -- get one. ON DELETE SET NULL — dropping the entity must never delete a trade record.
    entity_id     uuid,
    is_active     boolean     NOT NULL DEFAULT true,
    -- Symbol validity range (survivorship): when this listing began / was delisted.
    listed_on     date,
    delisted_on   date,
    created_at    timestamp with time zone DEFAULT now() NOT NULL,
    updated_at    timestamp with time zone DEFAULT now() NOT NULL,
    PRIMARY KEY (instrument_id),
    CONSTRAINT instruments_entity_fk FOREIGN KEY (entity_id)
        REFERENCES public.entities(id) ON DELETE SET NULL
);

-- One active listing per symbol; delisted rows are exempt so recycled tickers coexist.
CREATE UNIQUE INDEX instruments_active_symbol_uq
    ON public.instruments (symbol) WHERE is_active;
-- Typeahead: exact/prefix on symbol, trigram on symbol + name (pg_trgm from 00001).
CREATE INDEX instruments_symbol_lower_idx ON public.instruments (lower(symbol));
CREATE INDEX instruments_symbol_trgm_idx  ON public.instruments USING gin (symbol public.gin_trgm_ops);
CREATE INDEX instruments_name_trgm_idx    ON public.instruments USING gin (name public.gin_trgm_ops);
CREATE INDEX instruments_entity_idx       ON public.instruments (entity_id);

-- Seed a small curated US-equity universe so ticker search works before the Alpaca
-- syncer (T1) lands. Replaced wholesale once real reference data ingests; harmless.
INSERT INTO public.instruments (symbol, name, exchange) VALUES
    ('AAPL',  'Apple Inc.',                     'NASDAQ'),
    ('MSFT',  'Microsoft Corporation',          'NASDAQ'),
    ('NVDA',  'NVIDIA Corporation',             'NASDAQ'),
    ('GOOGL', 'Alphabet Inc. Class A',          'NASDAQ'),
    ('GOOG',  'Alphabet Inc. Class C',          'NASDAQ'),
    ('AMZN',  'Amazon.com, Inc.',               'NASDAQ'),
    ('META',  'Meta Platforms, Inc.',           'NASDAQ'),
    ('TSLA',  'Tesla, Inc.',                     'NASDAQ'),
    ('AMD',   'Advanced Micro Devices, Inc.',   'NASDAQ'),
    ('AVGO',  'Broadcom Inc.',                  'NASDAQ'),
    ('NFLX',  'Netflix, Inc.',                  'NASDAQ'),
    ('INTC',  'Intel Corporation',              'NASDAQ'),
    ('MU',    'Micron Technology, Inc.',        'NASDAQ'),
    ('QCOM',  'QUALCOMM Incorporated',          'NASDAQ'),
    ('CRM',   'Salesforce, Inc.',               'NYSE'),
    ('ORCL',  'Oracle Corporation',             'NYSE'),
    ('ADBE',  'Adobe Inc.',                     'NASDAQ'),
    ('PLTR',  'Palantir Technologies Inc.',     'NASDAQ'),
    ('SMCI',  'Super Micro Computer, Inc.',     'NASDAQ'),
    ('ARM',   'Arm Holdings plc',               'NASDAQ'),
    ('JPM',   'JPMorgan Chase & Co.',           'NYSE'),
    ('BAC',   'Bank of America Corporation',    'NYSE'),
    ('V',     'Visa Inc.',                      'NYSE'),
    ('MA',    'Mastercard Incorporated',        'NYSE'),
    ('BRK.B', 'Berkshire Hathaway Inc. Class B','NYSE'),
    ('UNH',   'UnitedHealth Group Incorporated','NYSE'),
    ('LLY',   'Eli Lilly and Company',          'NYSE'),
    ('JNJ',   'Johnson & Johnson',              'NYSE'),
    ('XOM',   'Exxon Mobil Corporation',        'NYSE'),
    ('CVX',   'Chevron Corporation',            'NYSE'),
    ('WMT',   'Walmart Inc.',                   'NYSE'),
    ('COST',  'Costco Wholesale Corporation',   'NASDAQ'),
    ('HD',    'The Home Depot, Inc.',           'NYSE'),
    ('DIS',   'The Walt Disney Company',        'NYSE'),
    ('KO',    'The Coca-Cola Company',          'NYSE'),
    ('PEP',   'PepsiCo, Inc.',                  'NASDAQ'),
    ('BA',    'The Boeing Company',             'NYSE'),
    ('UBER',  'Uber Technologies, Inc.',        'NYSE'),
    ('COIN',  'Coinbase Global, Inc.',          'NASDAQ'),
    ('SPY',   'SPDR S&P 500 ETF Trust',         'NYSE ARCA'),
    ('QQQ',   'Invesco QQQ Trust',              'NASDAQ'),
    ('IWM',   'iShares Russell 2000 ETF',       'NYSE ARCA');

-- +goose Down
DROP TABLE IF EXISTS public.instruments;
