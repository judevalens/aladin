# Local Ops Harness

The local ops harness is a Python CLI wrapped by `make` targets. It is for local development only: quick diagnostics, safe stream nudges, and recovery from stuck sync cycles.

## Commands

```sh
make ops-status
make ops-errors WINDOW=15m
make ops-streams
make ops-queues
make ops-force-stream PROVIDER=bluesky STREAM_KEY="ai agents"
make ops-reset-stuck-cycles AGE=30m
make worker-go CONCURRENCY=24
```

The direct CLI is:

```sh
python3 scripts/ops/aladin_ops.py status
python3 scripts/ops/aladin_ops.py errors --window 1h
python3 scripts/ops/aladin_ops.py streams
python3 scripts/ops/aladin_ops.py queues
python3 scripts/ops/aladin_ops.py force-stream --provider bluesky --stream-key "ai agents"
python3 scripts/ops/aladin_ops.py reset-stuck-cycles --age 30m
```

## Configuration

The harness loads `backend_v2/.env` and uses:

- `DATABASE_URL` for Postgres checks and safe DB updates.
- `REDIS_URL` for best-effort Asynq queue checks.
- `ALADIN_API_URL`, defaulting to `http://localhost:8000`.
- `LOKI_URL`, defaulting to `http://localhost:3100`.

It also shells out to local tools when present:

- `docker` for compose service status.
- `psql` for database checks and safe recovery commands.
- `redis-cli` for queue checks.
- `pgrep` for best-effort API/worker process checks.

Missing tools or unavailable services should degrade one section at a time. `ops-status` is expected to keep printing whatever it can.

Nango Cloud is the default provider-connection backend. Optional self-hosted Nango experiments run through a separate compose file:

```sh
make nango-up
make nango-logs
make nango-down
```

Self-hosted Nango uses its own Postgres and Redis containers and requires `NANGO_ENCRYPTION_KEY` plus `NANGO_SECRET_KEY` in the environment. See `docs/NANGO_PROVIDER_CONNECTIONS.md`.

## Safety Rules

The harness intentionally avoids destructive operations.

- `force-stream` only marks a matching provider stream due by clearing `last_refresh_at`, `last_picked_at`, and setting `sync_status = 'idle'`.
- `reset-stuck-cycles` only closes `sync_cycles` in `active` or `running` state older than the requested age.
- No command truncates tables, deletes source items, deletes records, or purges queues.
- Mutating commands print the affected row count from Postgres.

## Common Flows

Check whether the system is healthy:

```sh
make ops-status
```

Check whether recent failures are still active:

```sh
make ops-errors WINDOW=15m
make ops-errors WINDOW=1h
```

Force one Bluesky stream to fetch on the next scheduler tick:

```sh
make ops-force-stream PROVIDER=bluesky STREAM_KEY="ai agents"
make worker-go CONCURRENCY=24
```

Recover from old active/running cycles left by a killed worker:

```sh
make ops-reset-stuck-cycles AGE=30m
make ops-streams
```

## Observability Notes

Promtail parses JSON logs from `logs/*.log` and promotes low-cardinality labels:

- `level`
- `component`
- `task_type`
- `provider`

High-cardinality fields such as `correlation_id`, `record_id`, `source_item_id`, and `stream_key` stay in the log body and should be queried through text search rather than labels.
