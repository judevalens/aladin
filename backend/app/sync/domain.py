from __future__ import annotations
from dataclasses import dataclass, field
from datetime import datetime, timezone


def _now() -> datetime:
    return datetime.now(timezone.utc)


@dataclass
class RawArtifact:
    """Normalized artifact data returned by a syncer before DB persistence."""
    external_id: str
    type: str           # feed | post | comment | chunk | note | webpage | message | thread
    content: str
    label: str
    metadata: dict = field(default_factory=dict)
    source_url: str | None = None
    parent_external_id: str | None = None  # external_id of parent artifact (for tree structure)


@dataclass
class SyncJob:
    source_id: str
    source_type: str    # reddit | bluesky | insight
    job_type: str       # fetch_listing | fetch_thread | fetch_feed | insight_batch
    payload: dict = field(default_factory=dict)
    priority: int = 1
    scheduled_at: datetime = field(default_factory=_now)
    attempts: int = 0
    max_attempts: int = 3
    last_error: str | None = None
    snapshot_id: str | None = None
    id: str | None = None   # None until persisted


@dataclass
class SyncResult:
    artifacts: list[RawArtifact] = field(default_factory=list)
    next_job: SyncJob | None = None     # forward progress — pagination, scheduled re-fetch
    requeue_job: SyncJob | None = None  # retry — same job delayed on failure

    def __post_init__(self):
        if self.next_job and self.requeue_job:
            raise ValueError("SyncResult: only one of next_job or requeue_job may be set")
