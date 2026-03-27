# Aladin — Live Source Designs

## Sync Modes

Sources fall into three sync modes:

| Mode | How it works | Example sources |
|---|---|---|
| `poll` | Scheduler enqueues jobs on an interval. Worker fetches via HTTP. Cursor-based. | Reddit, Bluesky, Email |
| `push` | External system calls a webhook endpoint. No scheduler, no cursor. | Slack, GitHub, Linear |
| `one_shot` | Manual upload or paste. Single snapshot, never re-synced. | File upload, voice memo, paste |

The sync mode determines how artifacts are created. Everything downstream — enrichment, relevance scoring, graph promotion — is identical regardless of how an artifact arrived.

```
poll source:
  Scheduler → SyncJob → Queue → Worker → Syncer → fetch → Artifact (pending)

push source:
  External system → Webhook endpoint → Handler → Artifact (pending)

one_shot source:
  User action → Ingest endpoint → Artifact (pending)

                                          ↓ (all paths converge here)
                                   Enrichment pipeline
                                          ↓
                                   Graph / Insights
```

---

## Shared Syncer Interface (poll sources only)

Poll sources implement the syncer contract:

```python
class SourceSyncer(ABC):
    source_type: str  # 'reddit' | 'bluesky' | 'slack_poll'

    @abstractmethod
    def plan(self, source: Source) -> list[SyncJob]:
        """
        Called by scheduler. Returns initial jobs for this sync cycle.
        Example: Reddit returns one fetch_listing job + re-fetch jobs for active threads.
        """

    @abstractmethod
    def execute(self, job: SyncJob) -> SyncResult:
        """
        Called by worker. Executes one HTTP request.
        Returns artifacts + optional next_job (pagination) or requeue_job (failure).
        Owns its own rate limiting.
        """
```

```python
@dataclass
class SyncResult:
    artifacts: list[RawArtifact]
    next_job: SyncJob | None      # forward progress — new cursor, next page
    requeue_job: SyncJob | None   # same job retried — failure or rate limit
                                  # only one of next_job / requeue_job is set
```

Push sources do not implement this interface — they have a webhook handler instead.

---

## Source.config shapes

`sync_mode` determines what lives in `config`:

```
poll source config:
  cursor / after    ← resumption point, updated after each successful fetch
  credentials       ← API keys, tokens (encrypted at rest)
  filter params     ← min_score, channel_ids, search terms, etc.

push source config:
  signing_secret    ← webhook verification (e.g. Slack signing secret)
  channel_ids       ← which channels/topics to accept events from
  (no cursor — events arrive in real time, nothing to resume)

one_shot config:
  (empty or file metadata — no sync state needed)
```

---

## Reddit

### Source config

```
Source
  type: 'reddit'
  sync_mode: 'poll'
  config:
    subreddit: string           ← e.g. "MachineLearning"
    min_score: int              ← pre-pipeline noise gate: skip posts below this
    include_comments: bool      ← pull top-N comments as child artifacts
    top_comments_n: int         ← default 5
    after: string | null        ← cursor: Reddit fullname e.g. "t3_abc123"
    sort: 'new' | 'hot' | 'top' ← default 'new' for continuous sync
```

One Source per subreddit. A user tracking two subreddits creates two Sources.

### Artifact mapping

| Reddit unit | Artifact type | external_id | content |
|---|---|---|---|
| Post / thread | `message` | `t3_<post_id>` | post title + selftext |
| Comment (optional) | `message` | `t1_<comment_id>` | comment body |

```
Artifact (post)
  type: 'message'
  external_id: 't3_abc123'
  content: post.title + "\n\n" + post.selftext
  metadata:
    reddit_id: 't3_abc123'
    subreddit: 'MachineLearning'
    author: 'u/researcher'
    score: 847
    url: 'https://reddit.com/r/...'
    flair: 'Research'
    created_utc: 1710000000
    num_comments: 43

Artifact (comment, optional)
  type: 'message'
  external_id: 't1_xyz789'
  content: comment.body
  metadata:
    reddit_id: 't1_xyz789'
    parent_id: 't3_abc123'
    subreddit: 'MachineLearning'
    author: 'u/someone'
    score: 124
    created_utc: 1710001000
```

### Job types

| job_type | payload | what it does |
|---|---|---|
| `fetch_listing` | `{subreddit, cursor, stop_at_id, min_score}` | GET /new.json, paginate until stop_at_id |
| `fetch_thread` | `{post_id, subreddit, score_at_last_fetch, num_comments_at_last_fetch}` | re-fetch a post to detect edits/score changes/new comments |
| `fetch_comments` | `{post_id, subreddit}` | fetch top-N comments for a post |

### plan() logic

```python
def plan(self, source: Source) -> list[SyncJob]:
    jobs = []

    # 1. Fetch new posts from listing
    jobs.append(SyncJob(
        job_type='fetch_listing',
        payload={'subreddit': source.config['subreddit'], 'cursor': source.config.get('after'), ...},
        priority=1
    ))

    # 2. Re-fetch still-active threads based on score brackets
    active = get_active_threads(source.id)
    for thread in active:
        jobs.append(SyncJob(
            job_type='fetch_thread',
            payload={'post_id': thread.external_id, ...},
            priority=0,
            scheduled_at=thread.next_refetch_at
        ))

    return jobs
```

### Re-fetch schedule (score brackets)

```python
def next_refetch(score: int, age_hours: float) -> datetime | None:
    if score >= 500 and age_hours < 24:
        return now() + timedelta(minutes=30)
    if score >= 100 and age_hours < 12:
        return now() + timedelta(hours=1)
    if score >= 20 and age_hours < 6:
        return now() + timedelta(hours=2)
    return None  # don't re-enqueue — thread is cold
```

Thresholds are configurable per Source. A slow academic subreddit has different score distributions than a fast-moving one.

### Noise filtering (two-stage)

```
Stage 1 — pre-pipeline (source.config.min_score)
  Post score < min_score → never create Artifact
  Cheap. Runs in syncer before any DB write.

Stage 2 — post-enrichment (source.suggest_threshold)
  artifact.relevance_score < suggest_threshold → status: 'dismissed'
  Semantic similarity + entity overlap against existing KG.
  Expensive. Runs async in enrichment pipeline.
```

### Cursor semantics

Reddit's `after` param is a post fullname (`t3_xxx`). Stable — never changes. Points to the last post seen. On next listing fetch, resume just below it.

Pagination stops when the worker hits a `stop_at_id` (first post from previous sync cycle) or `data.after` is null.

---

## Bluesky

### Source config

```
Source
  type: 'bluesky'
  sync_mode: 'poll'          ← default; 'push' if using Jetstream
  config:
    feeds: string[]           ← feed generator URIs
    actor_dids: string[]      ← optional: specific accounts to follow
    cursor: string | null     ← opaque API cursor
    use_jetstream: bool       ← switch to firehose mode (push)
```

### Artifact mapping

| Bluesky unit | Artifact type | external_id |
|---|---|---|
| Post | `message` | `at://did:plc:.../app.bsky.feed.post/rkey` |
| Quote post | `message` | same URI, `metadata.quote_uri` set |

```
Artifact
  type: 'message'
  external_id: 'at://did:plc:abc/app.bsky.feed.post/3k...'
  content: post.record.text
  metadata:
    uri: 'at://...'
    cid: 'bafyrei...'
    author_did: 'did:plc:abc'
    author_handle: 'researcher.bsky.social'
    created_at: '2024-03-01T...'
    langs: ['en']
    like_count: 42
    repost_count: 8
    quote_uri: 'at://...' | null
    embed_type: 'image' | 'link' | 'video' | null
```

### Job types

| job_type | payload | what it does |
|---|---|---|
| `fetch_feed` | `{feed_uri, cursor}` | GET getFeed, one page |
| `fetch_thread` | `{post_uri}` | re-fetch post + replies to detect new replies / deletions |

### Poll vs Jetstream

**Polling (default, sync_mode: 'poll')**
- Scheduler-driven, cursor-based
- `getFeed` or `searchPosts` endpoint
- Good for topic feeds, manageable volume

**Jetstream (sync_mode: 'push')**
- WebSocket firehose, no scheduler
- Webhook handler processes events as they arrive
- cursor = `time_us` from last event, stored in `source.config.cursor` for reconnect
- Only for narrow actor-list sources — volume is too high for broad topic tracking

### Re-fetch schedule

Posts are largely immutable. Re-fetch only to detect:
- New replies (reply_count changed)
- Deletion (`NotFound` → mark artifact superseded)

```python
engagement = like_count + (repost_count * 3)

if engagement >= 100 and age_hours < 12:  return now() + timedelta(hours=1)
if engagement >= 20  and age_hours < 6:   return now() + timedelta(hours=2)
return None
```

---

## Slack (future, push)

```
Source
  type: 'slack'
  sync_mode: 'push'
  config:
    team_id: string
    channel_ids: string[]
    signing_secret: string    ← for webhook verification
```

Events arrive via Slack Events API webhook. No scheduler, no cursor. Each message event → Artifact directly.

Thread replies are fetched on-demand when a `message` event has `thread_ts` set.

---

## Sync Intervals (poll sources, defaults)

| Source type | Default interval | Rationale |
|---|---|---|
| Reddit | 60 min | Non-realtime, queue drains budget continuously |
| Bluesky (poll) | 30 min | Low urgency, feed generators handle curation |
| Email | 15 min | Lower urgency than Slack |
| File | on-upload | One-shot |

Intervals are per-source and configurable. The queue continuously drains the rate budget regardless of interval — idle time just means no work is queued.
