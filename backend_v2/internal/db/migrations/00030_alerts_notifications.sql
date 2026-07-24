-- +goose Up

-- Price alerts: recurring, self-re-arming threshold triggers on an instrument. Keyed by the
-- durable instrument_id (never symbol); symbol is denormalized so the alert engine's reconcile
-- can drive market-hub demand without a join. status stays 'active' across fires (recurring);
-- `armed` is the in-band slope/hysteresis gate — flips false on fire, true on a genuine pullback.
CREATE TABLE public.alerts (
    id               uuid PRIMARY KEY,
    user_id          uuid NOT NULL,
    instrument_id    uuid NOT NULL,
    symbol           text NOT NULL,
    direction        text NOT NULL,                  -- 'above' | 'below'
    threshold        numeric NOT NULL,
    armed            boolean NOT NULL DEFAULT true,
    status           text NOT NULL DEFAULT 'active',  -- 'active' | 'paused'
    last_fired_at    timestamptz,
    last_fired_price numeric,
    created_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT alerts_direction_chk CHECK (direction IN ('above','below')),
    CONSTRAINT alerts_status_chk    CHECK (status IN ('active','paused')),
    CONSTRAINT alerts_instrument_fk FOREIGN KEY (instrument_id)
        REFERENCES public.instruments(instrument_id) ON DELETE CASCADE
);
-- List a user's alerts (newest first).
CREATE INDEX alerts_user_idx ON public.alerts (user_id, created_at DESC);
-- Reconcile load: all active alerts (grouped by instrument) to rebuild the in-memory index
-- and drive market-hub demand.
CREATE INDEX alerts_active_instrument_idx ON public.alerts (instrument_id) WHERE status = 'active';

-- Notifications: the reusable durable per-user inbox. Alerts are the first producer
-- (kind='price_alert'); future producers (insights, fills, copilot) reuse the same table +
-- realtime event + FE surface. Durability lives here — the outbox app_event is the live toast.
CREATE TABLE public.notifications (
    id         uuid PRIMARY KEY,
    user_id    uuid NOT NULL,
    kind       text NOT NULL,                     -- 'price_alert' (extensible)
    title      text NOT NULL,
    body       text NOT NULL DEFAULT '',
    data       jsonb NOT NULL DEFAULT '{}'::jsonb, -- {alertId, instrumentId, symbol, direction, threshold, price}
    read_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
-- Inbox list (newest first) + the unread badge.
CREATE INDEX notifications_user_idx ON public.notifications (user_id, created_at DESC);
CREATE INDEX notifications_unread_idx ON public.notifications (user_id) WHERE read_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS public.notifications;
DROP TABLE IF EXISTS public.alerts;
