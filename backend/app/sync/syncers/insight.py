from __future__ import annotations
import logging

from ..base import SourceSyncer
from ..domain import SyncJob, SyncResult
from ...db.models import Source, Snapshot

logger = logging.getLogger(__name__)


class InsightSyncer(SourceSyncer):
    """
    Stub — insight batch pipeline not yet implemented (Phase 4).
    Accepts insight_batch jobs and logs them without failing.
    """
    source_type = "insight"

    def plan(self, source: Source, snapshot: Snapshot) -> list[SyncJob]:
        return []

    def execute(self, job: SyncJob) -> SyncResult:
        logger.info(
            "Insight batch for snapshot %s — pipeline not yet implemented, skipping",
            job.payload.get("snapshot_id")
        )
        return SyncResult()
