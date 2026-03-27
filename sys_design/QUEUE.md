# Aladin — Sync Queue Design

## Overview

The sync queue is a Postgres-backed job queue. It drives all poll-based source syncing. Push sources bypass it entirely — their events arrive via webhook and write artifacts directly.

Three processes interact with the queue:

- **Scheduler** — finds sources due for sync, calls `syncer.plan()`, enqueues jobs
- **Worker** — dequeues jobs, routes to the correct syncer, handles results
- **Router** — dispatches jobs to the correct syncer based on `source_type`

The worker is source-agnostic. All source-specific logic lives in syncers.

---

## Job Schema

```sql
CREATE TABLE sync_jobs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id     UUID NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    source_type   TEXT NOT NULL,                  -- denormalized for router — no join needed
    job_type      TEXT NOT NULL,                  -- syncer-specific: fetch_listing | fetch_thread | fetch_feed | insight_batch
    payload       JSONB NOT NULL DEFAULT '{}',
    priority      INT NOT NULL DEFAULT 1,
    scheduled_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    attempts      INT NOT NULL DEFAULT 0,
    max_attempts  INT NOT NULL DEFAULT 3,
    last_error    TEXT,
    status        TEXT NOT NULL DEFAULT 'pending'  -- pending | processing | dead
);

CREATE INDEX idx_sync_jobs_dequeue
    ON sync_jobs (priority DESC, scheduled_at ASC, created_at ASC)
    WHERE status = 'pending';
```

### Status transitions

```
pending    → processing   worker dequeues (SKIP LOCKED)
processing → deleted      job succeeded — deleted on completion
processing → pending      requeue_job returned — new row inserted, old deleted
processing → dead         attempts >= max_attempts
```

Completed jobs are deleted. Dead jobs are kept for inspection.

### Priority levels

| Priority | Use case |
|---|---|
| 10 | New source backfill — user waiting for initial data |
| 5 | Insight batch — snapshot complete, insights pending |
| 1 | Routine sync — fetch_listing, fetch_feed |
| 0 | Background fill — thread re-fetches, comment fetches |

---

## Job Payload Shapes

```
fetch_listing (Reddit)
  subreddit: string
  cursor: string | null         ← null on first fetch
  stop_at_id: string | null     ← fullname of most recent post from last sync
  min_score: int

fetch_thread (Reddit)
  post_id: string               ← fullname e.g. "t3_abc123"
  subreddit: string
  score_at_last_fetch: int
  num_comments_at_last_fetch: int

fetch_feed (Bluesky)
  feed_uri: string
  cursor: string | null

fetch_thread (Bluesky)
  post_uri: string              ← AT URI

insight_batch
  snapshot_id: string
  kg_id: string
  artifact_ids: [string]        ← all enriched artifact IDs from this snapshot
```

---

## Domain Objects

```python
@dataclass
class SyncJob:
    source_id:    str
    source_type:  str
    job_type:     str
    payload:      dict
    priority:     int = 1
    scheduled_at: datetime = field(default_factory=datetime.utcnow)
    attempts:     int = 0
    max_attempts: int = 3
    last_error:   str | None = None
    id:           str | None = None   # None until persisted

@dataclass
class SyncResult:
    artifacts:    list[RawArtifact]
    next_job:     SyncJob | None    # forward progress — pagination, scheduled re-fetch
    requeue_job:  SyncJob | None    # retry — same job delayed
                                    # only one of next_job / requeue_job is set
```

---

## Syncer Interface

```python
class SourceSyncer(ABC):
    source_type: str

    @abstractmethod
    def plan(self, source: Source, snapshot: Snapshot) -> list[SyncJob]:
        """
        Called by scheduler. Returns jobs to enqueue for this sync cycle.
        Sets snapshot.expected_jobs = len(returned jobs).
        """

    @abstractmethod
    def execute(self, job: SyncJob) -> SyncResult:
        """
        Called by worker. One HTTP request.
        Returns artifacts + optional next_job or requeue_job.
        Owns its own rate limiting.
        """
```

---

## Job Queue — Builder Pattern

```python
queue = (
    JobQueue
    .builder()
    .add(RedditSyncer())
    .add(BlueskySyncer())
    .build()
)

queue.run()   # starts scheduler + worker loops
```

The builder accumulates syncers, wires them into the router, and returns a configured `JobQueue` instance ready to run.

```python
class JobQueue:
    def __init__(self, router: SyncRouter):
        self.router = router

    @staticmethod
    def builder() -> 'JobQueueBuilder':
        return JobQueueBuilder()

    def run(self):
        # starts scheduler thread + worker loop
        ...

class JobQueueBuilder:
    def __init__(self):
        self._syncers: list[SourceSyncer] = []

    def add(self, syncer: SourceSyncer) -> 'JobQueueBuilder':
        self._syncers.append(syncer)
        return self

    def build(self) -> JobQueue:
        router = SyncRouter({s.source_type: s for s in self._syncers})
        return JobQueue(router)
```

---

## Worker Loop

```python
def run_worker(router: SyncRouter):
    while True:
        job = dequeue()

        if not job:
            sleep(1)
            continue

        result = router.route(job)
        upsert_artifacts(result.artifacts)
        on_job_complete(job)          # increments snapshot.completed_jobs

        if result.next_job:
            enqueue(result.next_job)
            delete_job(job.id)

        elif result.requeue_job:
            enqueue(result.requeue_job)
            delete_job(job.id)

        else:
            delete_job(job.id)
```

### on_job_complete

```python
def on_job_complete(job: SyncJob):
    snapshot = get_snapshot_for_job(job)
    if not snapshot:
        return

    snapshot.completed_jobs += 1

    if snapshot.completed_jobs >= snapshot.expected_jobs:
        snapshot.status = 'complete'
        # trigger insight batch
        enqueue(SyncJob(
            source_id=job.source_id,
            source_type='insight',
            job_type='insight_batch',
            payload={'snapshot_id': snapshot.id, 'kg_id': snapshot.source.kg_id},
            priority=5
        ))
```

### Dequeue query

```sql
SELECT * FROM sync_jobs
WHERE status = 'pending'
  AND scheduled_at <= now()
ORDER BY priority DESC, scheduled_at ASC, created_at ASC
FOR UPDATE SKIP LOCKED
LIMIT 1
```

---

## Router

```python
class SyncRouter:
    def __init__(self, syncers: dict[str, SourceSyncer]):
        self.syncers = syncers

    def route(self, job: SyncJob) -> SyncResult:
        syncer = self.syncers.get(job.source_type)
        if not syncer:
            raise ValueError(f"No syncer for source_type: {job.source_type}")
        return syncer.execute(job)
```

---

## Scheduler

```python
def run_scheduler(router: SyncRouter):
    while True:
        sources = db.query("""
            SELECT * FROM sources
            WHERE sync_mode = 'poll'
              AND sync_state = 'active'
              AND next_sync_at <= now()
            FOR UPDATE SKIP LOCKED
        """)

        for source in sources:
            snapshot = create_snapshot(source)          # status: processing
            syncer = router.syncers[source.type]
            jobs = syncer.plan(source, snapshot)
            snapshot.expected_jobs = len(jobs)
            enqueue_all(jobs)
            source.next_sync_at = now() + source.sync_interval

        sleep(60)
```

---

## Rate Limiting

Each syncer owns its rate limit in-process. The worker does not throttle — it dequeues as fast as possible. The syncer blocks internally before making HTTP requests.

```python
class RedditSyncer(SourceSyncer):
    source_type = 'reddit'
    _last_request_at: float = 0
    _min_interval: float = 60 / 55   # 55 req/min

    def _wait(self):
        elapsed = time.monotonic() - self._last_request_at
        gap = self._min_interval - elapsed
        if gap > 0:
            time.sleep(gap)
        self._last_request_at = time.monotonic()

    def execute(self, job: SyncJob) -> SyncResult:
        self._wait()
        # ... fetch
```

**Constraint:** rate limiting is in-memory. Run one worker process only. Revisit when scale demands shared state (Redis token bucket, or per-source-type queues).

---

## Error Handling

```
429 Too Many Requests:
  → requeue_job: scheduled_at = now() + exponential_backoff(attempts)
  → backoff: 30s, 2m, 10m, 1h (max)
  → attempts++

5xx Server Error:
  → requeue_job with backoff
  → attempts++

attempts >= max_attempts:
  → SyncResult(artifacts=[], next_job=None, requeue_job=None)
  → job marked dead

3 consecutive dead jobs for same source:
  → source.sync_state = 'error'
  → surface to user for re-activation
```

---

## Future: scaling

Current: single worker process, in-memory rate limiting, Postgres queue.

When scale demands it:
- Rate limit state → Redis shared bucket → multiple workers safe
- Queue → Redis (BullMQ) or Flink for stream processing
- Per-source-type queues with dedicated workers

Syncer interface and job schema don't change — only the queue backend and concurrency model.
