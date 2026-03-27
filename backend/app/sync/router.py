from __future__ import annotations
from .base import SourceSyncer
from .domain import SyncJob, SyncResult


class SyncRouter:
    """
    Routes a dequeued job to the correct syncer based on source_type.
    Completely source-agnostic — add new source types by registering a new syncer.
    """

    def __init__(self, syncers: dict[str, SourceSyncer]):
        self._syncers = syncers

    def route(self, job: SyncJob) -> SyncResult:
        syncer = self._syncers.get(job.source_type)
        if syncer is None:
            raise ValueError(
                f"No syncer registered for source_type '{job.source_type}'. "
                f"Registered: {list(self._syncers.keys())}"
            )
        return syncer.execute(job)

    @property
    def syncers(self) -> dict[str, SourceSyncer]:
        return self._syncers
