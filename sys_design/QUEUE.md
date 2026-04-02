# Aladin — Sync Queue Design

## Overview

The sync queue drives all poll-based source syncing. It is backed by **asynq**
(Redis-based task queue), replacing the earlier Postgres-backed queue.

Three components interact with the queue:

- **Scheduler** — finds sources due for sync, calls `syncer.Plan()`, enqueues jobs
- **Worker** — asynq processes jobs, routes to the correct syncer, produces artifacts
- **Pipeline handoff** — each artifact is enqueued as a `pipeline:ingest` task for the blackboard

The worker is source-agnostic. All source-specific logic lives in syncers.
The asynq server, client, and mux are owned by `cmd/worker/main.go` and shared
across sync and pipeline handlers.

---

## Architecture

```
Scheduler (signal-based)
  └─→ planAndEnqueue() → asynq.Enqueue("sync:<type>")
        └─→ Syncer.Execute() → SyncResult.Artifacts
              └─→ asynq.Enqueue("pipeline:ingest") per artifact
                    └─→ Blackboard IngestHandler → ready channel → pipeline
```

The scheduler is signal-based, not ticker-based. A `chan Signal` replaces the
polling loop. Two signal sources exist:
- `Trigger(sourceID)` — called immediately when a new source is created
- Recurring ticker — fires every hour to schedule all due sources

---

## Task Types

| Task type        | Queue    | Producer          | Consumer |
|------------------|----------|-------------------|----------|
| `sync:<type>`    | varies   | Scheduler         | Per-syncer handler |
| `pipeline:ingest`| default  | Syncer handler    | Blackboard IngestHandler |
| `pipeline:retry` | default  | Pipeline controller | RetryHandler |

### Queue priorities

| Queue    | Weight | Use case |
|----------|--------|----------|
| critical | 6      | New source backfill (user waiting) |
| default  | 3      | Routine sync + pipeline tasks |
| low      | 1      | Background thread re-fetches |

---

## Syncer Interface (Go)

```go
type Syncer interface {
    SourceType() string
    Plan(ctx context.Context, source db.Source) ([]*db.SyncJob, error)
    Execute(ctx context.Context, job *db.SyncJob) (*SyncResult, error)
}

type SyncResult struct {
    Artifacts []*RawArtifact
    NextJob   *db.SyncJob   // pagination / scheduled re-fetch
}
```

`Plan` is called by the scheduler. It returns jobs to enqueue for this sync cycle.
`Execute` is called by the asynq worker. One HTTP request per call. Returns
artifacts + optional `NextJob` for pagination chains.

---

## Job Schema

```go
type SyncJob struct {
    ID          string
    SourceID    string
    KgID        string
    SnapshotID  string
    SourceType  string
    JobType     string
    Payload     map[string]any
    Priority    int
    Attempts    int
    MaxAttempts int
    LastError   string
}
```

Jobs are serialized as JSON in asynq task payloads. asynq handles persistence,
retry scheduling, and dead-letter in Redis.

---

## Scheduler (Signal-based)

```go
func (q *Queue) Start(ctx context.Context) error {
    go q.runTicker(ctx)     // fires Signal{} every hour
    go q.runScheduler(ctx)  // consumes signals
    return nil
}

func (q *Queue) Trigger(sourceID string) {
    q.signals <- Signal{SourceID: sourceID}  // immediate sync on source creation
}
```

On a `Signal{}` (no SourceID): query `sources WHERE next_sync_at <= now()`, plan and enqueue for each.

On a `Signal{SourceID}`: fetch that source, plan and enqueue immediately.

---

## Syncer Handler (asynq)

```go
// Registered on the shared asynq.ServeMux:
mux.HandleFunc("sync:reddit", queue.makeHandler(redditSyncer))

func (q *Queue) makeHandler(syncer Syncer) asynq.HandlerFunc {
    return func(ctx context.Context, t *asynq.Task) error {
        var job db.SyncJob
        json.Unmarshal(t.Payload(), &job)

        result, err := syncer.Execute(ctx, &job)

        // Enqueue pipeline:ingest per artifact
        for _, a := range result.Artifacts {
            payload, _ := json.Marshal(blackboard.IngestPayload{...})
            q.client.Enqueue(asynq.NewTask("pipeline:ingest", payload))
        }

        // Chain next page if present
        if result.NextJob != nil {
            q.enqueue(result.NextJob)
        }
        return nil
    }
}
```

---

## Error Handling

asynq handles retry scheduling for sync jobs natively via `asynq.MaxRetry`.
Pipeline-level errors (rate limits, transient failures) are handled separately
by the pipeline controller using `ErrRateLimit` / `ErrTransient` / `ErrPermanent`
typed errors and `pipeline:retry` tasks.

---

## Registering Syncers

```go
queue := isync.NewQueue(asynqClient, sourceRepo,
    syncers.NewRedditSyncer(),
    // syncers.NewTwitterSyncer(),
)
queue.RegisterHandlers(mux)  // registers "sync:<type>" on shared mux
queue.Start(ctx)
```

---

## Rate Limiting

Each syncer is responsible for its own rate limiting via a shared `ratelimit.Limiter`
(token bucket). The asynq worker does not throttle — it processes as fast as asynq
dequeues. The syncer blocks before making HTTP requests.

| Source  | Limit       |
|---------|-------------|
| Reddit  | ~55 req/min |
| OpenAI  | 60 RPM (pipeline) |
| Tavily  | 20 RPM (pipeline) |

---

## Observability

All sync operations log structured JSON with trace fields:

```json
{ "source_id": "...", "source_type": "reddit", "job_type": "fetch_listing", "artifact_count": 42 }
```

Logs are shipped to Loki via Promtail. Grafana dashboard available at port 3001.

---

## Scaling Path

Current: single worker process.

When scale demands it:
- Multiple worker processes: asynq is safe for concurrent consumers — Redis handles coordination
- Per-source-type queues: add dedicated asynq queues per source type with separate workers
- Higher throughput: replace `pipeline:ingest` with Kafka; syncer interface unchanged

The syncer interface and job schema don't change — only the queue backend.
