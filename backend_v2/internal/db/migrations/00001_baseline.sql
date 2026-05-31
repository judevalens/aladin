-- +goose Up

-- Squashed baseline (data-layer-model branch). Replaces migrations 00001-00029,
-- which had accumulated heavy build-then-modify-then-drop churn (notably the
-- abandoned sync redesign: workspace_changes / canonical sync columns, all
-- superseded by the generic outbox_events log). Generated from the live v29
-- schema via pg_dump --schema-only. Safe to squash: no deployed environment
-- tracks the pre-squash goose versions and the branch is unpushed.
-- No function bodies / $$ blocks, so goose's default ;-splitter applies.

SET statement_timeout = 0;

SET lock_timeout = 0;

SET idle_in_transaction_session_timeout = 0;

SET client_encoding = 'UTF8';

SET standard_conforming_strings = on;

SET check_function_bodies = false;

SET xmloption = content;

SET client_min_messages = warning;

SET row_security = off;

CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;

CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public;

SET default_tablespace = '';

SET default_table_access_method = heap;

CREATE TABLE public.artifacts (
    id text NOT NULL,
    user_id uuid NOT NULL,
    type text NOT NULL,
    title text NOT NULL,
    content text NOT NULL,
    summary text,
    source_url text,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE public.chunks (
    id uuid NOT NULL,
    document_id text NOT NULL,
    page integer,
    text text NOT NULL,
    embedding public.vector(1536)
);

CREATE TABLE public.documents (
    id text NOT NULL,
    filename text NOT NULL,
    title text,
    size integer,
    page_count integer,
    uploaded_at timestamp with time zone NOT NULL,
    ingested boolean NOT NULL,
    uploaded_by uuid
);

CREATE TABLE public.files (
    id text NOT NULL,
    user_id uuid NOT NULL,
    storage_key text NOT NULL,
    uploaded_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE public.insights (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    kg_id uuid NOT NULL,
    type text NOT NULL,
    title text NOT NULL,
    body text NOT NULL,
    record_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    entity text,
    topic text,
    confidence double precision DEFAULT '0.8'::double precision NOT NULL,
    user_status text DEFAULT 'pending'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now()
);

CREATE TABLE public.integration_tokens (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    name text NOT NULL,
    token_hash text NOT NULL,
    scopes text[] DEFAULT '{}'::text[] NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    expires_at timestamp with time zone,
    revoked_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    last_used_at timestamp with time zone
);

CREATE TABLE public.knowledge_graphs (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    name text NOT NULL,
    description text,
    pipeline_autonomy text DEFAULT 'suggest'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE public.outbox_events (
    xid xid8 DEFAULT pg_current_xact_id() NOT NULL,
    user_id uuid NOT NULL,
    type text NOT NULL,
    payload jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE public.page_documents (
    artifact_id text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    revision bigint DEFAULT 0 NOT NULL,
    blocks jsonb DEFAULT '[]'::jsonb NOT NULL,
    search_text text DEFAULT ''::text NOT NULL,
    last_collab_commit_at timestamp with time zone
);

CREATE TABLE public.page_ydoc (
    page_id text NOT NULL,
    state bytea NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE public.provider_connections (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    provider text NOT NULL,
    backend text DEFAULT 'nango'::text NOT NULL,
    provider_config_key text NOT NULL,
    external_connection_id text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    granted_scopes text[] DEFAULT '{}'::text[] NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    disconnected_at timestamp with time zone
);

CREATE TABLE public.provider_streams (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    provider text NOT NULL,
    stream_kind text NOT NULL,
    stream_key text NOT NULL,
    name text NOT NULL,
    visibility text DEFAULT 'public'::text NOT NULL,
    sync_mode text DEFAULT 'poll'::text NOT NULL,
    sync_state text DEFAULT 'active'::text NOT NULL,
    sync_status text DEFAULT 'idle'::text NOT NULL,
    config jsonb DEFAULT '{}'::jsonb NOT NULL,
    last_picked_at timestamp with time zone,
    last_refresh_at timestamp with time zone,
    last_sync_started_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_user_id uuid,
    provider_connection_id uuid
);

CREATE TABLE public.records (
    id text NOT NULL,
    type text NOT NULL,
    label text NOT NULL,
    content text NOT NULL,
    source_url text,
    enrichment jsonb,
    external_id text,
    version integer DEFAULT 1 NOT NULL,
    superseded_by text,
    embedding public.vector(1536),
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    relevance_score double precision,
    status text DEFAULT 'pending'::text NOT NULL,
    parent_id text,
    user_status text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    source_revision bigint DEFAULT 0 NOT NULL,
    provider text,
    provider_stream_id uuid,
    owner_user_id uuid
);

CREATE TABLE public.source_subscriptions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    kg_id uuid NOT NULL,
    provider_stream_id uuid NOT NULL,
    name text NOT NULL,
    visibility text DEFAULT 'public'::text NOT NULL,
    policy jsonb DEFAULT '{}'::jsonb NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE public.sync_cycles (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    kind text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    cursor jsonb DEFAULT '{}'::jsonb NOT NULL,
    head_boundary jsonb DEFAULT '{}'::jsonb NOT NULL,
    last_picked_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone,
    completion_reason text,
    last_hydrated_at timestamp with time zone,
    target_kind text DEFAULT ''::text NOT NULL,
    target_external_id text DEFAULT ''::text NOT NULL,
    provider_stream_id uuid
);

CREATE TABLE public.tenant_item_matches (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    subscription_id uuid NOT NULL,
    source_revision bigint NOT NULL,
    match_source text NOT NULL,
    overlap_entities jsonb DEFAULT '[]'::jsonb NOT NULL,
    relevance_status text DEFAULT 'pending'::text NOT NULL,
    relevance_score double precision,
    relevance_reason text,
    record_id text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE public.tree_nodes (
    id text NOT NULL,
    user_id uuid NOT NULL,
    parent_id text,
    kind text NOT NULL,
    title text,
    artifact_id text,
    "position" bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    seq bigint DEFAULT 0 NOT NULL,
    is_deleted boolean DEFAULT false NOT NULL,
    CONSTRAINT tree_nodes_check CHECK ((((kind = 'folder'::text) AND (artifact_id IS NULL) AND (title IS NOT NULL)) OR ((kind = 'artifact'::text) AND (artifact_id IS NOT NULL)))),
    CONSTRAINT tree_nodes_kind_check CHECK ((kind = ANY (ARRAY['folder'::text, 'artifact'::text])))
);

CREATE TABLE public.user_sessions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    token_hash text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    revoked_at timestamp with time zone,
    user_agent text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    last_seen_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE public.users (
    id uuid NOT NULL,
    email text NOT NULL,
    created_at timestamp with time zone NOT NULL,
    password_hash text,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    last_login_at timestamp with time zone
);

ALTER TABLE ONLY public.records
    ADD CONSTRAINT artifacts_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.artifacts
    ADD CONSTRAINT artifacts_pkey1 PRIMARY KEY (id);

ALTER TABLE ONLY public.chunks
    ADD CONSTRAINT chunks_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.documents
    ADD CONSTRAINT documents_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.files
    ADD CONSTRAINT files_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.insights
    ADD CONSTRAINT insights_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.integration_tokens
    ADD CONSTRAINT integration_tokens_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.integration_tokens
    ADD CONSTRAINT integration_tokens_token_hash_key UNIQUE (token_hash);

ALTER TABLE ONLY public.knowledge_graphs
    ADD CONSTRAINT knowledge_graphs_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.page_documents
    ADD CONSTRAINT page_documents_pkey PRIMARY KEY (artifact_id);

ALTER TABLE ONLY public.page_ydoc
    ADD CONSTRAINT page_ydoc_pkey PRIMARY KEY (page_id);

ALTER TABLE ONLY public.provider_connections
    ADD CONSTRAINT provider_connections_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.provider_streams
    ADD CONSTRAINT provider_streams_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.source_subscriptions
    ADD CONSTRAINT source_subscriptions_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.source_subscriptions
    ADD CONSTRAINT source_subscriptions_user_id_kg_id_provider_stream_id_key UNIQUE (user_id, kg_id, provider_stream_id);

ALTER TABLE ONLY public.sync_cycles
    ADD CONSTRAINT sync_cycles_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.tenant_item_matches
    ADD CONSTRAINT tenant_item_matches_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.tree_nodes
    ADD CONSTRAINT tree_nodes_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.user_sessions
    ADD CONSTRAINT user_sessions_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.user_sessions
    ADD CONSTRAINT user_sessions_token_hash_key UNIQUE (token_hash);

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);

CREATE INDEX idx_artifacts_type ON public.artifacts USING btree (type);

CREATE INDEX idx_artifacts_user_updated ON public.artifacts USING btree (user_id, updated_at DESC);

CREATE INDEX idx_files_user_uploaded ON public.files USING btree (user_id, uploaded_at DESC);

CREATE INDEX idx_insights_kg ON public.insights USING btree (kg_id);

CREATE INDEX idx_insights_user_status ON public.insights USING btree (user_status);

CREATE INDEX idx_integration_tokens_active_hash ON public.integration_tokens USING btree (token_hash) WHERE ((status = 'active'::text) AND (revoked_at IS NULL));

CREATE INDEX idx_integration_tokens_user ON public.integration_tokens USING btree (user_id, created_at DESC);

CREATE INDEX idx_outbox_user_xid ON public.outbox_events USING btree (user_id, xid);

CREATE INDEX idx_page_documents_search_text_trgm ON public.page_documents USING gin (search_text public.gin_trgm_ops);

CREATE INDEX idx_provider_connections_user_active ON public.provider_connections USING btree (user_id, updated_at DESC) WHERE (status = 'active'::text);

CREATE INDEX idx_provider_streams_owner_connection ON public.provider_streams USING btree (owner_user_id, provider_connection_id) WHERE (owner_user_id IS NOT NULL);

CREATE INDEX idx_records_parent ON public.records USING btree (parent_id) WHERE (parent_id IS NOT NULL);

CREATE INDEX idx_records_pending ON public.records USING btree (created_at) WHERE (status = 'pending'::text);

CREATE INDEX idx_records_pipeline_status ON public.records USING btree (status, created_at) WHERE (status = ANY (ARRAY['pending'::text, 'enriched'::text, 'embedded'::text]));

CREATE INDEX idx_records_user_status ON public.records USING btree (user_status) WHERE (user_status IS NOT NULL);

CREATE INDEX idx_source_subscriptions_stream_active ON public.source_subscriptions USING btree (provider_stream_id) WHERE (status = 'active'::text);

CREATE INDEX idx_sync_cycles_provider_stream_active ON public.sync_cycles USING btree (provider_stream_id, created_at) WHERE (status = ANY (ARRAY['active'::text, 'running'::text]));

CREATE INDEX idx_tenant_item_matches_subscription ON public.tenant_item_matches USING btree (subscription_id, created_at DESC);

CREATE UNIQUE INDEX idx_tree_nodes_user_artifact ON public.tree_nodes USING btree (user_id, artifact_id) WHERE (artifact_id IS NOT NULL);

CREATE INDEX idx_tree_nodes_user_parent_kind ON public.tree_nodes USING btree (user_id, parent_id, kind, "position");

CREATE UNIQUE INDEX idx_tree_nodes_user_parent_position ON public.tree_nodes USING btree (user_id, COALESCE(parent_id, ''::text), "position") WHERE (is_deleted = false);

CREATE INDEX idx_user_sessions_active ON public.user_sessions USING btree (token_hash, expires_at) WHERE (revoked_at IS NULL);

CREATE INDEX idx_user_sessions_user ON public.user_sessions USING btree (user_id, created_at DESC);

CREATE UNIQUE INDEX uq_provider_connections_active ON public.provider_connections USING btree (user_id, backend, provider_config_key, external_connection_id) WHERE (status = 'active'::text);

CREATE UNIQUE INDEX uq_provider_streams_private_owner ON public.provider_streams USING btree (provider, stream_kind, stream_key, owner_user_id) WHERE (owner_user_id IS NOT NULL);

CREATE UNIQUE INDEX uq_provider_streams_public ON public.provider_streams USING btree (provider, stream_kind, stream_key) WHERE (owner_user_id IS NULL);

CREATE UNIQUE INDEX uq_sync_cycles_provider_stream_active_target ON public.sync_cycles USING btree (provider_stream_id, kind, target_kind, target_external_id) WHERE ((status = ANY (ARRAY['active'::text, 'running'::text])) AND (target_external_id IS NOT NULL));

CREATE UNIQUE INDEX uq_tenant_item_matches_subscription_record_revision ON public.tenant_item_matches USING btree (subscription_id, record_id, source_revision);

ALTER TABLE ONLY public.records
    ADD CONSTRAINT artifacts_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES public.records(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.records
    ADD CONSTRAINT artifacts_superseded_by_fkey FOREIGN KEY (superseded_by) REFERENCES public.records(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.artifacts
    ADD CONSTRAINT artifacts_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.chunks
    ADD CONSTRAINT chunks_document_id_fkey FOREIGN KEY (document_id) REFERENCES public.documents(id);

ALTER TABLE ONLY public.documents
    ADD CONSTRAINT documents_uploaded_by_fkey FOREIGN KEY (uploaded_by) REFERENCES public.users(id);

ALTER TABLE ONLY public.files
    ADD CONSTRAINT files_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.insights
    ADD CONSTRAINT insights_kg_id_fkey FOREIGN KEY (kg_id) REFERENCES public.knowledge_graphs(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.integration_tokens
    ADD CONSTRAINT integration_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.knowledge_graphs
    ADD CONSTRAINT knowledge_graphs_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.outbox_events
    ADD CONSTRAINT outbox_events_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.page_documents
    ADD CONSTRAINT page_documents_artifact_id_fkey FOREIGN KEY (artifact_id) REFERENCES public.artifacts(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.page_ydoc
    ADD CONSTRAINT page_ydoc_page_id_fkey FOREIGN KEY (page_id) REFERENCES public.artifacts(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.provider_connections
    ADD CONSTRAINT provider_connections_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.provider_streams
    ADD CONSTRAINT provider_streams_owner_user_id_fkey FOREIGN KEY (owner_user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.provider_streams
    ADD CONSTRAINT provider_streams_provider_connection_id_fkey FOREIGN KEY (provider_connection_id) REFERENCES public.provider_connections(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.records
    ADD CONSTRAINT records_owner_user_id_fkey FOREIGN KEY (owner_user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.records
    ADD CONSTRAINT records_provider_stream_id_fkey FOREIGN KEY (provider_stream_id) REFERENCES public.provider_streams(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.source_subscriptions
    ADD CONSTRAINT source_subscriptions_kg_id_fkey FOREIGN KEY (kg_id) REFERENCES public.knowledge_graphs(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.source_subscriptions
    ADD CONSTRAINT source_subscriptions_provider_stream_id_fkey FOREIGN KEY (provider_stream_id) REFERENCES public.provider_streams(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.source_subscriptions
    ADD CONSTRAINT source_subscriptions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.sync_cycles
    ADD CONSTRAINT sync_cycles_provider_stream_id_fkey FOREIGN KEY (provider_stream_id) REFERENCES public.provider_streams(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.tenant_item_matches
    ADD CONSTRAINT tenant_item_matches_record_id_fkey FOREIGN KEY (record_id) REFERENCES public.records(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.tenant_item_matches
    ADD CONSTRAINT tenant_item_matches_subscription_id_fkey FOREIGN KEY (subscription_id) REFERENCES public.source_subscriptions(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.tree_nodes
    ADD CONSTRAINT tree_nodes_artifact_id_fkey FOREIGN KEY (artifact_id) REFERENCES public.artifacts(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.tree_nodes
    ADD CONSTRAINT tree_nodes_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES public.tree_nodes(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.tree_nodes
    ADD CONSTRAINT tree_nodes_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.user_sessions
    ADD CONSTRAINT user_sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

-- +goose Down

-- Rolling back the baseline drops everything (an empty database).
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
