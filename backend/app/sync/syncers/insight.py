from __future__ import annotations
import logging

from ..base import SourceSyncer
from ..domain import SyncJob, SyncResult
from ...db.models import Source, Snapshot

logger = logging.getLogger(__name__)


class InsightSyncer(SourceSyncer):
    """
    Runs insight generation after a snapshot completes.
    Triggered automatically by the job queue when a sync cycle finishes.
    """
    source_type = "insight"

    def plan(self, source: Source, snapshot: Snapshot) -> list[SyncJob]:
        return []

    def execute(self, job: SyncJob) -> SyncResult:
        kg_id       = job.payload.get("kg_id")
        snapshot_id = job.payload.get("snapshot_id")

        if not kg_id:
            logger.warning("Insight job missing kg_id — skipping")
            return SyncResult()

        logger.info("Running insight generation for kg %s (snapshot %s)", kg_id, snapshot_id)

        try:
            from app.pipeline.insights import generate_and_store
            count = generate_and_store(kg_id=kg_id, snapshot_id=snapshot_id)
            logger.info("Generated %d insights for kg %s", count, kg_id)
        except Exception:
            logger.exception("Insight generation failed for kg %s", kg_id)

        return SyncResult()
