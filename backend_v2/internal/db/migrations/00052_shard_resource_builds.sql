-- +goose Up
CREATE TABLE shard_resource_builds (
 user_id uuid NOT NULL,
 shard_id text NOT NULL,
 environment text NOT NULL,
 build_id text NOT NULL,
 files jsonb NOT NULL CHECK (jsonb_typeof(files) = 'object'),
 PRIMARY KEY (user_id,shard_id,environment,build_id),
 FOREIGN KEY (user_id,shard_id,environment,build_id)
 REFERENCES shard_resource_releases(user_id,shard_id,environment,build_id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE shard_resource_builds;
