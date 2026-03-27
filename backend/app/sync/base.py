from __future__ import annotations
from abc import ABC, abstractmethod
from .domain import SyncJob, SyncResult
from ..db.models import Source, Snapshot


class SourceSyncer(ABC):
    """
    Implemented once per source type. Owns:
    - plan():    what jobs to enqueue for a sync cycle
    - execute(): one HTTP request → artifacts + optional follow-up job
    - rate limiting (in-process, per syncer instance)
    """

    source_type: str  # must be set on subclass

    @abstractmethod
    def plan(self, source: Source, snapshot: Snapshot) -> list[SyncJob]:
        """
        Called by the scheduler at the start of a sync cycle.
        Returns the initial set of jobs to enqueue.
        Sets snapshot.expected_jobs implicitly via len(returned list).
        """

    @abstractmethod
    def execute(self, job: SyncJob) -> SyncResult:
        """
        Called by the worker. Executes one HTTP request.
        Returns artifacts + optional next_job (pagination) or requeue_job (failure).
        Must handle its own rate limiting before making the request.
        """
