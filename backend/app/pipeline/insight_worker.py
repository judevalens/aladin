"""
Insight worker — generates insights when new enriched artifacts are available.

Driven by insight_queue: PipelineWorker pushes kg_ids after enrichment,
this worker blocks until items arrive then runs generate_and_store per KG.
"""
from __future__ import annotations
import logging
import queue
import threading

logger = logging.getLogger(__name__)


class InsightWorker:
    """Background thread that generates insights whenever new artifacts are enriched."""

    def start(self):
        t = threading.Thread(target=self._run, name="insight-worker", daemon=True)
        t.start()
        logger.info("Insight worker started")

    def run_once(self):
        """Drain whatever is queued right now and run. Useful for CLI/make."""
        from app.pipeline.insight_queue import drain
        kg_ids = drain()
        if not kg_ids:
            # Nothing queued — fall back to all KGs with enriched artifacts
            kg_ids = self._all_enriched_kg_ids()
        self._process(kg_ids)

    def _run(self):
        from app.pipeline.insight_queue import wait_and_drain
        while True:
            try:
                kg_ids = wait_and_drain(timeout=60.0)
                self._process(kg_ids)
            except queue.Empty:
                pass  # timeout — loop and wait again
            except Exception:
                logger.exception("Insight worker error")

    def _process(self, kg_ids: set[str]):
        from app.pipeline.insights import generate_and_store
        for kg_id in kg_ids:
            try:
                count = generate_and_store(kg_id=kg_id)
                if count:
                    logger.info("Generated %d insights for kg %s", count, kg_id)
            except Exception:
                logger.exception("Insight generation failed for kg %s", kg_id)

    def _all_enriched_kg_ids(self) -> set[str]:
        from sqlalchemy import text
        from app.db import get_session
        with get_session() as session:
            rows = session.execute(text("""
                SELECT DISTINCT s.kg_id::text
                FROM artifacts a
                JOIN sources s ON s.id = a.source_id
                WHERE a.enrichment IS NOT NULL
                  AND a.status NOT IN ('superseded', 'dismissed')
            """)).fetchall()
        return {r[0] for r in rows}
