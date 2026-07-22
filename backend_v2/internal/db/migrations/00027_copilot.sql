-- +goose Up
-- The Copilot — the default agentic LLM interface in Aladin. A thread is one
-- conversation; messages are the visible turns (user + final assistant answer).
-- Intra-turn tool calls/results are ephemeral (streamed, not stored) — only the
-- durable conversation lives here so the dock survives reload and a reconnecting
-- client can replay history. Ids are generated app-side (google/uuid), matching
-- the rest of the codebase.
CREATE TABLE public.copilot_threads (
    id         uuid PRIMARY KEY,
    user_id    uuid NOT NULL,
    title      text NOT NULL DEFAULT '',
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);
CREATE INDEX copilot_threads_user_idx ON public.copilot_threads (user_id, updated_at DESC);

CREATE TABLE public.copilot_messages (
    id         uuid PRIMARY KEY,
    thread_id  uuid NOT NULL,
    -- seq is the monotonic insertion order WITHIN a thread's turns. created_at is not
    -- reliable for ordering: two turns of one round can share the same now(), and a uuid
    -- tiebreaker is not insertion order. Order the conversation by seq.
    seq        bigint GENERATED ALWAYS AS IDENTITY,
    role       text NOT NULL,                             -- 'user' | 'assistant'
    content    text NOT NULL DEFAULT '',
    -- Grounding citations the assistant used, as [{kind,id,title}]. Empty for user turns.
    citations  jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT copilot_messages_thread_fk FOREIGN KEY (thread_id)
        REFERENCES public.copilot_threads(id) ON DELETE CASCADE
);
CREATE INDEX copilot_messages_thread_idx ON public.copilot_messages (thread_id, seq);

-- +goose Down
DROP TABLE IF EXISTS public.copilot_messages;
DROP TABLE IF EXISTS public.copilot_threads;
