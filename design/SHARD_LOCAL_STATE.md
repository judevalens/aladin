# Shard Local State

## Summary

Shard local app state should start as a simple Postgres-backed key/value document
store:

- one shard owns many small JSON documents
- each document is addressed by a stable string key
- each key has its own revision, so conflicts are granular
- writes emit realtime events through the existing outbox path
- the shard iframe never talks to storage directly; it uses the Aladin bridge

This gives shards a document-store feel without introducing a second database or
reimplementing Firebase.

## What Belongs Here

Use shard local state for UI/app data that belongs to one generated shard:

- filters
- layouts
- selected rows or active tabs
- simulation inputs
- scenario settings
- chart marks
- view-local annotations
- widget state
- viewer preferences scoped to the shard

Do not use it for canonical Aladin knowledge:

- sources
- records
- insights
- entity facts
- durable claims
- anything that should be citeable, searchable, or visible outside the shard

If local shard data becomes workspace knowledge, promote it into the entity layer.

## Data Model

```sql
CREATE TABLE public.shard_kv (
    shard_id   text NOT NULL,
    channel    text NOT NULL,
    key        text NOT NULL,
    value      jsonb NOT NULL DEFAULT '{}'::jsonb,
    revision   bigint NOT NULL DEFAULT 0,
    created_by uuid,
    updated_by uuid,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    deleted_at timestamp with time zone,
    PRIMARY KEY (shard_id, channel, key)
);

CREATE INDEX shard_kv_prefix_idx
    ON public.shard_kv (shard_id, channel, key text_pattern_ops);

CREATE INDEX shard_kv_updated_idx
    ON public.shard_kv (shard_id, channel, updated_at DESC);
```

`shard_id` points at an artifact with `type = 'app'`.

`channel` is usually `draft` or `published`.

`key` is an application-level path. It should be stable, human-readable, and
prefix-queryable.

`value` is the JSON payload for that key.

`revision` is the optimistic concurrency guard for that key only.

`deleted_at` allows tombstone-style deletes when realtime subscribers need to see
the deletion before compaction.

## Example Keys

```text
settings
filters
layout/main
scenario/base
scenario/stress
annotations/aapl
chart/SPY/marks
viewer/user_123/preferences
```

Keys should be treated as shard-local namespaced paths. Prefixes become the
subscription boundary:

```text
annotations/
scenario/
chart/SPY/
viewer/user_123/
```

## Bridge API

The shard iframe should use a bridge API, not direct HTTP or direct database
access:

```ts
shard.kv.get(key)
shard.kv.list(prefix)
shard.kv.set(key, value, baseRevision)
shard.kv.patch(key, patch, baseRevision)
shard.kv.delete(key, baseRevision)
shard.kv.subscribe(prefix, handler)
```

The bridge is responsible for:

- enforcing shard ownership/access
- scoping all calls to the current shard and channel
- attaching the viewer/user identity
- preserving the sandbox boundary
- translating writes into backend API calls
- routing realtime events back into the iframe

## Write Semantics

All writes should be optimistic and revision-guarded.

Set flow:

1. Client reads `key` at `revision = N`.
2. Client edits locally.
3. Client calls `set(key, value, N)`.
4. Server verifies the current revision is still `N`.
5. Server writes the new value and bumps revision to `N + 1`.
6. Server appends an outbox event in the same transaction.
7. Other viewers receive the update over realtime.

If the stored revision has changed, return `409 Conflict` with the latest value
and revision.

```json
{
  "error": "conflict",
  "key": "filters",
  "currentRevision": 12,
  "currentValue": {}
}
```

This keeps conflict handling simple and local to one key.

## Realtime Events

Use the existing outbox pattern. A shard key write and its realtime event should
commit atomically.

Suggested payload:

```json
{
  "shardId": "shd_123",
  "channel": "draft",
  "key": "annotations/aapl",
  "operation": "set",
  "revision": 3,
  "value": {}
}
```

Suggested operations:

```text
set
patch
delete
```

The existing realtime resource can either reuse `artifact`:

```text
artifact.shard-kv-set
artifact.shard-kv-patch
artifact.shard-kv-delete
```

or introduce a dedicated `shard` resource kind:

```text
shard.kv-set
shard.kv-patch
shard.kv-delete
```

Reusing `artifact` is the smaller local change because shard build status already
uses artifact-scoped realtime events.

## Query Shape

The v1 query API should stay intentionally small:

```text
get exact key
list by prefix
list updated since timestamp/revision
```

Avoid turning shard local state into a general query engine. If a shard needs
complex relational querying, that data is probably no longer just local UI state.

## Why Not One Big Snapshot

A single `snapshot jsonb` row is simpler, but every concurrent edit conflicts
with every other edit. Key-based JSON keeps the same simplicity while making
conflicts granular:

```text
client A edits filters
client B edits annotations/aapl
both can succeed independently
```

It also makes subscriptions cheaper. A viewer can subscribe to `annotations/`
without receiving unrelated layout or scenario changes.

## Why Not Mongo Yet

MongoDB would fit granular shard-local documents and provides change streams for
reactive updates. But introducing Mongo means adding a second truth store,
separate backup/restore, cross-store permission checks, and a new realtime source.

Postgres key/value JSON gets most of the document-store benefits while staying
inside the current architecture:

- existing artifact ownership
- existing transactions
- existing outbox
- existing realtime fanout
- existing operational model

Mongo remains a good future option if shard local state becomes high-volume,
deeply document-oriented, or needs more document-native indexing.

## Future Extensions

Possible additions after v1:

- JSON Patch support for partial updates
- per-key history table for undo/debugging
- TTL-backed presence state
- compaction for deleted tombstones
- per-key schema hints in the shard manifest
- migration of hot collections to a document database
- CRDT/Yjs-backed keys for truly collaborative widgets

The bridge API should stay stable enough that storage can evolve underneath it.
