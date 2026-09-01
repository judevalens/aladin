-- V2 uses the existing PostgreSQL/JSONB storage engine in an isolated namespace.
-- Legacy /kv endpoints cannot address these rows. No legacy data is migrated.
-- +goose Up
CREATE TABLE shard_resource_releases (
 user_id uuid NOT NULL,
 shard_id text NOT NULL,
 environment text NOT NULL CHECK (environment IN ('draft','published')),
 build_id text NOT NULL,
 contract_hash text NOT NULL,
 generation text NOT NULL,
 contract_source bytea NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (user_id, shard_id, environment, build_id)
);
CREATE TABLE shard_resource_active (
 user_id uuid NOT NULL,
 shard_id text NOT NULL,
 environment text NOT NULL CHECK (environment IN ('draft','published')),
 build_id text NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (user_id, shard_id, environment),
 FOREIGN KEY (user_id, shard_id, environment, build_id)
  REFERENCES shard_resource_releases(user_id, shard_id, environment, build_id)
);
CREATE TABLE shard_resource_records (
 user_id uuid NOT NULL,
 shard_id text NOT NULL,
 environment text NOT NULL CHECK (environment IN ('draft','published')),
 generation text NOT NULL,
 dataset_id text NOT NULL,
 id text COLLATE "C" NOT NULL,
 schema_version bigint NOT NULL CHECK (schema_version > 0),
 revision bigint NOT NULL CHECK (revision > 0),
 data jsonb NOT NULL CHECK (jsonb_typeof(data)='object'),
 data_bytes bigint NOT NULL CHECK (data_bytes >= 0),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 created_by text NOT NULL,
 updated_by text NOT NULL,
 deleted_at timestamptz,
 PRIMARY KEY (user_id, shard_id, environment, generation, dataset_id, id)
);
CREATE TABLE shard_resource_receipts (
 user_id uuid NOT NULL,
 actor_key text NOT NULL,
 shard_id text NOT NULL,
 environment text NOT NULL,
 request_id text NOT NULL,
 payload_hash text NOT NULL,
 outcome jsonb NOT NULL,
 outcome_bytes bigint NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL,
 PRIMARY KEY (user_id, actor_key, shard_id, environment, request_id)
);
CREATE INDEX shard_resource_receipts_expiry ON shard_resource_receipts(expires_at);
-- An opaque cursor is a short random token, bound to an exact authorized view.
CREATE TABLE shard_resource_cursors (
 token uuid PRIMARY KEY,
 user_id uuid NOT NULL,
 actor_key text NOT NULL,
 shard_id text NOT NULL,
 environment text NOT NULL,
 view_hash text NOT NULL,
 page_offset integer NOT NULL CHECK (page_offset >= 0),
 expires_at timestamptz NOT NULL
);
CREATE INDEX shard_resource_cursors_scope ON shard_resource_cursors(user_id, shard_id, environment, expires_at);

-- +goose Down
DROP TABLE shard_resource_cursors;
DROP TABLE shard_resource_receipts;
DROP TABLE shard_resource_records;
DROP TABLE shard_resource_active;
DROP TABLE shard_resource_releases;
