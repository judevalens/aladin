# Pipeline v2 — Blackboard FSM Architecture

## Overview

An event-driven blackboard pipeline for artifact enrichment. Redis holds FSM
state and all intermediate data for in-flight artifacts. Postgres is written
once — at the very end, when the artifact is fully processed. Each stage is a
single worker goroutine consuming from a dedicated channel.

---

## Transport Layer

Syncers are decoupled from the pipeline via asynq (Redis-backed task queue).

```
Syncer (fetch_listing, fetch_thread, etc.)
  └─→ enqueues pipeline:ingest task per artifact
        └─→ IngestHandler: writes blackboard entry → pushes to ready channel
              └─→ Controller router dispatches to stage channel
```

This gives the pipeline an effectively unbounded input buffer with no back-pressure
on the syncer. The syncer completes its fetch regardless of pipeline throughput.

---

## Stores

| Store    | Responsibility |
|----------|----------------|
| Redis    | Blackboard — FSM state + full raw artifact + intermediate results |
| asynq    | Transport — pipeline:ingest, pipeline:retry task queues |
| Postgres | Sink — written once at StateComplete with all data |

---

## FSM States

```
PENDING
  └─→ FIRST_PASS_RUNNING
        └─→ FIRST_PASS_DONE
              ├─→ (no low-confidence entities) EMBEDDING_PENDING
              └─→ SEARCHING
                    └─→ EMBEDDING_PENDING
                          └─→ EMBEDDING_RUNNING
                                └─→ GRAPH_PENDING
                                      └─→ GRAPH_RUNNING
                                            └─→ COMPLETE → persist to PG, delete from Redis
```

Failures are handled by the controller, not tracked as FSM states. On error:
- `ErrRateLimit` → schedule `pipeline:retry` task via `asynq.ProcessIn(retryAfter)`
- `ErrTransient` → exponential backoff, increment `entry.Attempts`, schedule retry
- `ErrPermanent` → delete entry from Redis, no PG write

---

## Blackboard Entry (Redis JSON)

Key: `pipeline:artifact:<artifact_id>`

```json
{
  "artifact_id": "uuid",
  "kg_id": "uuid",
  "state": "searching",
  "attempts": 0,
  "max_attempts": 3,
  "retry_at": null,
  "updated_at": "2026-01-01T00:00:00Z",

  "raw": {
    "external_id": "t3_abc123",
    "source_id": "uuid",
    "type": "post",
    "label": "Title of post",
    "content": "Full text...",
    "source_url": "https://reddit.com/r/.../",
    "metadata": {}
  },

  "first_pass": {
    "summary": "...",
    "entities": ["Anthropic", "Claude"],
    "topics": ["AI", "LLMs"],
    "key_claims": ["..."],
    "low_confidence_entities": ["Cognition AI"]
  },

  "search": {
    "pending": ["Cognition AI"],
    "resolved": {
      "Anthropic": [{ "title": "...", "url": "...", "content": "..." }]
    }
  },

  "embedding": [0.1, 0.2, ...]
}
```

The full raw artifact is stored in Redis so workers never need to read from PG.

---

## Controller

Event-driven — no ticker. One goroutine per stage.

```
ready chan (buf=4)
  └─→ router goroutine
        ├─→ fpCh   → FirstPassWorker goroutine
        ├─→ srchCh → SearchWorker goroutine
        ├─→ embedCh → EmbedWorker goroutine
        └─→ graphCh → GraphWorker goroutine
              └─→ (on success) push back to ready
                    └─→ router sees COMPLETE → persist() → done
```

On startup, the controller drains any in-flight Redis entries back into `ready`
(crash recovery). Workers update the blackboard entry before starting work so
recovery knows which state to resume from.

---

## Workers

### FirstPassWorker
- **Input state**: `PENDING`
- **Action**: call OpenAI GPT-4o-mini, extract summary/entities/topics/key_claims/low_confidence_entities
- **Output state**: `FIRST_PASS_DONE`
- **PG write**: none

### SearchWorker
- **Input state**: `SEARCHING`
- **Action**: resolve each pending entity via Tavily (Redis-cached)
- **On rate limit**: return `ErrRateLimit{RetryAfter: 30s}`
- **Output state**: `EMBEDDING_PENDING`
- **PG write**: none

### EmbedWorker
- **Input state**: `EMBEDDING_PENDING`
- **Action**: call OpenAI text-embedding-3-small
- **Output**: stores vector in `entry.Embedding` (Redis), transitions to `GRAPH_PENDING`
- **PG write**: none

### GraphWorker
- **Input state**: `GRAPH_PENDING`
- **Action**: call `GraphPromoter.Promote()` — MERGE artifact/entity/topic nodes in Neo4j
- **Output state**: `COMPLETE`
- **PG write**: none (controller persists after COMPLETE)

### Controller persist (at COMPLETE)
- Single `INSERT ... ON CONFLICT (source_id, external_id) DO NOTHING`
- Writes: id, source_id, external_id, type, label, content, source_url, metadata, enrichment (jsonb), embedding (vector), status='in_graph'
- Then signals insight worker with `kg_id`
- Then deletes blackboard entry

---

## Error Handling

Typed errors from workers drive controller retry logic:

| Error type      | Action |
|-----------------|--------|
| `ErrRateLimit`  | Schedule `pipeline:retry` task with `asynq.ProcessIn(retryAfter)` |
| `ErrTransient`  | Increment attempts, exponential backoff (30s→2m→8m→max 1h), schedule retry |
| `ErrPermanent`  | Delete from blackboard — unrecoverable |
| unknown         | Treated as transient |

`pipeline:retry` handler: fetches entry from Redis, pushes back to `ready` channel.

Backoff formula: `min(30 * 4^attempts seconds, 3600s)`

---

## Rate Limiting

Token bucket, shared across workers that hit the same API.

| Service | Limit  | Shared by |
|---------|--------|-----------|
| OpenAI  | 60 RPM | FirstPassWorker + EmbedWorker |
| Tavily  | 20 RPM | SearchWorker |
| Neo4j   | none   | GraphWorker |

---

## Search Cache

Key: `search:<query_hash>`
TTL: 7 days

Redis sits in front of Tavily. Cache hit skips the Tavily call entirely.

---

## Postgres Write Summary

| When | What is written |
|------|----------------|
| `StateComplete` (once) | Full artifact: content, metadata, enrichment, embedding, status='in_graph' |

All prior stages are zero-PG-write. Old design's per-stage PG writes are gone.

---

## asynq Task Types

| Task type         | Producer        | Consumer |
|-------------------|-----------------|----------|
| `pipeline:ingest` | Syncer handler  | `IngestHandler` → ready channel |
| `pipeline:retry`  | Controller      | `RetryHandler` → ready channel |
| `sync:<type>`     | Scheduler       | Per-syncer handler |

---

## File Structure

```
backend_v2/
  cmd/worker/main.go              — single binary: asynq server + controller + insight worker

  internal/
    pipeline/
      controller.go               — ready channel, router, worker pools, persist, recover
      retry.go                    — pipeline:retry asynq handler
      workers/
        interface.go              — Worker interface
        errors.go                 — ErrRateLimit, ErrTransient, ErrPermanent
        first_pass.go
        search.go
        embed.go
        graph.go
      blackboard/
        state.go                  — FSM states, Entry, SyncedArtifact, FirstPassResult, SearchState
        blackboard.go             — Redis CRUD + IngestHandler

    graph/
      promoter.go                 — Neo4j promoter (Artifact, Entity, Topic MERGE)

    sync/
      queue.go                    — signal-based scheduler, asynq handler registration
      syncers/
        reddit.go

    db/
      models.go                   — CompletedArtifact, EmbeddedArtifact, Source, SyncJob, etc.
      artifact_repo.go            — SaveComplete only
      source_repo.go
      repositories.go

    search/
      tavily.go
      cache.go

    ratelimit/
      limiter.go
```

---

## Scaling Path

Current design runs one worker process. To scale:

- **Multiple workers**: replace in-process ready channel with a Redis stream or additional asynq queue type. Controller becomes stateless — workers pull from queue directly.
- **Per-stage scaling**: each stage becomes its own asynq queue + worker pool. Controller becomes a pure router.
- **Higher throughput**: switch to Kafka for artifact transport. Syncer interface and worker logic stay unchanged — only the handoff changes.
