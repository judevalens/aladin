"""
Pipeline worker — runs in a background thread alongside the sync queue.

Processes artifacts through the intelligence lifecycle:
  pending → enriched → embedded → in_graph

Skips:
  - feed artifacts (containers, not meaningful content)
  - very short content (< 30 chars)
  - superseded artifacts
"""
from __future__ import annotations
import logging
import threading
import time
from datetime import datetime, timezone

from sqlalchemy import text

from app.db import get_session

logger = logging.getLogger(__name__)

POLL_INTERVAL   = 5   # seconds between scans
BATCH_SIZE      = 10  # max artifacts per tick
MIN_CONTENT_LEN = 30  # skip trivially short content

# Thread-safe stats for health endpoint
_stats = {
    "enriched":  0,
    "embedded":  0,
    "promoted":  0,
    "errors":    0,
    "last_tick": None,
}
_stats_lock = threading.Lock()


def get_stats() -> dict:
    with _stats_lock:
        return dict(_stats)


def _inc(key: str):
    with _stats_lock:
        _stats[key] += 1


# Types that are containers — skip enrichment
_SKIP_TYPES = {"feed"}


class PipelineWorker:
    """Background thread that advances artifacts through the pipeline."""

    def start(self):
        t = threading.Thread(target=self._run, name="pipeline-worker", daemon=True)
        t.start()
        logger.info("Pipeline worker started")

    def _run(self):
        while True:
            try:
                self._tick()
            except Exception:
                logger.exception("Pipeline worker error")
            time.sleep(POLL_INTERVAL)

    def _tick(self):
        with _stats_lock:
            _stats["last_tick"] = datetime.now(timezone.utc).isoformat()

        self._process_pending()
        self._process_enriched()
        self._process_embedded()

    # ── Stage 1: enrich pending artifacts ──────────────────────────────────

    def _process_pending(self):
        with get_session() as session:
            rows = session.execute(text("""
                SELECT a.id, a.type, a.label, a.content, a.source_url,
                       s.kg_id::text AS kg_id
                FROM artifacts a
                JOIN sources s ON s.id = a.source_id
                WHERE a.status = 'pending'
                  AND a.type NOT IN ('feed')
                  AND char_length(a.content) >= :min_len
                ORDER BY a.created_at ASC
                LIMIT :batch
            """), {"min_len": MIN_CONTENT_LEN, "batch": BATCH_SIZE}).fetchall()

        for row in rows:
            self._enrich_artifact(row.id, row.type, row.label, row.content, row.source_url, row.kg_id)

    def _enrich_artifact(self, artifact_id: str, artifact_type: str,
                         label: str, content: str, source_url: str | None, kg_id: str):
        try:
            from app.pipeline.enrichment import enrich
            from app.pipeline.insight_queue import push as push_insight
            result = enrich(content=content, artifact_type=artifact_type)
            enrichment = result.model_dump()

            with get_session() as session:
                session.execute(text("""
                    UPDATE artifacts
                    SET enrichment = CAST(:enrichment AS jsonb), status = 'enriched'
                    WHERE id = :id AND status = 'pending'
                """), {"enrichment": _json(enrichment), "id": artifact_id})
                session.commit()

            _inc("enriched")
            logger.debug("Enriched artifact %s", artifact_id)
            push_insight(kg_id)
        except Exception as e:
            _inc("errors")
            logger.error("Enrichment failed for %s: %s", artifact_id, e)

    # ── Stage 2: embed enriched artifacts ──────────────────────────────────

    def _process_enriched(self):
        with get_session() as session:
            rows = session.execute(text("""
                SELECT id, label, content, enrichment
                FROM artifacts
                WHERE status = 'enriched'
                  AND char_length(content) >= :min_len
                ORDER BY created_at ASC
                LIMIT :batch
            """), {"min_len": MIN_CONTENT_LEN, "batch": BATCH_SIZE}).fetchall()

        for row in rows:
            self._embed_artifact(row.id, row.label, row.content, row.enrichment or {})

    def _embed_artifact(self, artifact_id: str, label: str, content: str, enrichment: dict):
        try:
            from app.pipeline.embedding import embed_text

            # Embed label + summary + content for richer retrieval
            summary = (enrichment.get("summary") or "")
            text_to_embed = f"{label}\n{summary}\n{content}"
            vector = embed_text(text_to_embed)

            with get_session() as session:
                session.execute(text("""
                    UPDATE artifacts
                    SET embedding = CAST(:vec AS vector), status = 'embedded'
                    WHERE id = :id AND status = 'enriched'
                """), {"vec": str(vector), "id": artifact_id})
                session.commit()

            _inc("embedded")
            logger.debug("Embedded artifact %s", artifact_id)
        except Exception as e:
            _inc("errors")
            logger.error("Embedding failed for %s: %s", artifact_id, e)

    # ── Stage 3: promote embedded artifacts to Neo4j ───────────────────────

    def _process_embedded(self):
        with get_session() as session:
            rows = session.execute(text("""
                SELECT id, type, label, source_url, enrichment
                FROM artifacts
                WHERE status = 'embedded'
                ORDER BY created_at ASC
                LIMIT :batch
            """), {"batch": BATCH_SIZE}).fetchall()

        for row in rows:
            self._promote_artifact(row.id, row.type, row.label,
                                   row.source_url, row.enrichment or {})

    def _promote_artifact(self, artifact_id: str, artifact_type: str,
                          label: str, source_url: str | None, enrichment: dict):
        try:
            from app.pipeline.graph_promoter import promote
            promote(artifact_id, label, artifact_type, source_url, enrichment)

            with get_session() as session:
                session.execute(text("""
                    UPDATE artifacts SET status = 'in_graph'
                    WHERE id = :id AND status = 'embedded'
                """), {"id": artifact_id})
                session.commit()

            _inc("promoted")
        except Exception as e:
            _inc("errors")
            logger.error("Graph promotion failed for %s: %s", artifact_id, e)


def _json(d: dict) -> str:
    import json
    return json.dumps(d)
