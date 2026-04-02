from __future__ import annotations
import logging
import time
import threading
import uuid
from datetime import datetime, timezone, timedelta

from sqlalchemy import text
from sqlalchemy.orm import Session

from .base import SourceSyncer
from .domain import SyncJob, SyncResult, RawArtifact
from .router import SyncRouter
from ..db import get_session
from ..db.models import SyncJob as SyncJobModel, Snapshot, Source, Artifact

logger = logging.getLogger(__name__)

# How long a scheduler sleeps between checking for due sources (seconds)
SCHEDULER_INTERVAL = 60

# How long the worker sleeps when the queue is empty (seconds)
WORKER_IDLE_SLEEP = 1


class JobQueue:
    """
    Postgres-backed job queue.

    Usage:
        queue = (
            JobQueue
            .builder()
            .add(RedditSyncer())
            .add(BlueskySyncer())
            .build()
        )
        queue.run()
    """

    def __init__(self, router: SyncRouter):
        self.router = router

    @staticmethod
    def builder() -> JobQueueBuilder:
        return JobQueueBuilder()

    def run(self):
        """Start scheduler + one worker thread per syncer. Blocks until interrupted."""
        scheduler_thread = threading.Thread(
            target=self._run_scheduler, name="sync-scheduler", daemon=True
        )
        scheduler_thread.start()
        logger.info("Scheduler started")

        for source_type in self.router.syncers:
            t = threading.Thread(
                target=self._run_worker,
                args=(source_type,),
                name=f"worker-{source_type}",
                daemon=True,
            )
            t.start()
            logger.info("Worker started for source_type=%s", source_type)

        # Block main thread until interrupted
        threading.Event().wait()

    # ── Scheduler ────────────────────────────────────────────────────────────

    def _run_scheduler(self):
        while True:
            try:
                self._scheduler_tick()
            except Exception:
                logger.exception("Scheduler error")
            time.sleep(SCHEDULER_INTERVAL)

    def _scheduler_tick(self):
        with get_session() as session:
            # Clean up snapshots stuck in processing for > 2 hours
            stuck = session.execute(text("""
                UPDATE snapshots
                SET status = 'failed',
                    metadata = metadata || '{"failure_reason": "timeout"}'::jsonb
                WHERE status = 'processing'
                  AND created_at < now() - interval '2 hours'
                RETURNING id, source_id
            """)).fetchall()

            for row in stuck:
                logger.warning(
                    "Snapshot %s for source %s timed out — marked failed",
                    row.id, row.source_id
                )
                # Dead-letter any pending jobs for this snapshot
                session.execute(text("""
                    UPDATE sync_jobs
                    SET status = 'dead', last_error = 'snapshot timed out'
                    WHERE snapshot_id = :snapshot_id
                      AND status IN ('pending', 'processing')
                """), {"snapshot_id": str(row.id)})

            if stuck:
                session.commit()

            sources = session.execute(text("""
                SELECT * FROM sources
                WHERE sync_mode = 'poll'
                  AND sync_state = 'active'
                  AND (next_sync_at IS NULL OR next_sync_at <= now())
                FOR UPDATE SKIP LOCKED
            """)).fetchall()

            for row in sources:
                source = session.get(Source, row.id)
                syncer = self.router.syncers.get(source.type)
                if syncer is None:
                    logger.warning("No syncer for source type '%s', skipping", source.type)
                    continue

                # Don't start a new cycle if previous snapshot is still processing
                active = session.execute(text("""
                    SELECT id FROM snapshots
                    WHERE source_id = :source_id
                      AND status = 'processing'
                    LIMIT 1
                """), {"source_id": str(source.id)}).fetchone()

                if active:
                    logger.info(
                        "Source %s has an active snapshot — skipping until complete",
                        source.name
                    )
                    continue

                snapshot = self._create_snapshot(session, source)
                jobs = syncer.plan(source, snapshot)

                snapshot.expected_jobs = len(jobs)
                if snapshot.expected_jobs == 0:
                    snapshot.status = "complete"

                for job in jobs:
                    job.snapshot_id = str(snapshot.id)
                    _enqueue(session, job)

                source.next_sync_at = datetime.now(timezone.utc) + _sync_interval(source)
                session.commit()

                logger.info(
                    "Scheduled %d jobs for source %s (snapshot v%d)",
                    len(jobs), source.name, snapshot.version
                )

    def _create_snapshot(self, session: Session, source: Source) -> Snapshot:
        last = session.execute(text(
            "SELECT MAX(version) FROM snapshots WHERE source_id = :sid"
        ), {"sid": str(source.id)}).scalar()

        snapshot = Snapshot(
            id=uuid.uuid4(),
            source_id=source.id,
            version=(last or 0) + 1,
            status="processing",
        )
        session.add(snapshot)
        session.flush()
        return snapshot

    # ── Worker ───────────────────────────────────────────────────────────────

    def _run_worker(self, source_type: str):
        while True:
            try:
                processed = self._worker_tick(source_type)
                if not processed:
                    time.sleep(WORKER_IDLE_SLEEP)
            except Exception:
                logger.exception("Worker error (source_type=%s)", source_type)
                time.sleep(WORKER_IDLE_SLEEP)

    def _worker_tick(self, source_type: str) -> bool:
        """Dequeue and process one job for the given source type. Returns True if a job was processed."""
        with get_session() as session:
            row = session.execute(text("""
                SELECT * FROM sync_jobs
                WHERE status = 'pending'
                  AND source_type = :source_type
                  AND scheduled_at <= now()
                ORDER BY priority DESC, scheduled_at ASC, created_at ASC
                FOR UPDATE SKIP LOCKED
                LIMIT 1
            """), {"source_type": source_type}).fetchone()

            if row is None:
                return False

            # Mark as processing
            session.execute(text(
                "UPDATE sync_jobs SET status = 'processing' WHERE id = :id"
            ), {"id": str(row.id)})
            session.commit()

        # Execute outside transaction — syncer may be slow / blocking
        job = _row_to_job(row)
        try:
            result = self.router.route(job)
        except Exception as exc:
            self._handle_failure(job, str(exc))
            return True

        self._handle_result(job, result)
        return True

    def _handle_result(self, job: SyncJob, result: SyncResult):
        with get_session() as session:
            # Persist artifacts
            for raw in result.artifacts:
                _upsert_artifact(session, job, raw)

            # Follow-up jobs
            if result.next_job:
                result.next_job.snapshot_id = job.snapshot_id
                _enqueue(session, result.next_job)
            elif result.requeue_job:
                result.requeue_job.snapshot_id = job.snapshot_id
                _enqueue(session, result.requeue_job)

            # Snapshot completion — only when no more pages/follow-ups for this chain
            is_terminal = result.next_job is None and result.requeue_job is None
            _on_job_complete(session, job, is_terminal)

            # Delete completed job
            session.execute(text(
                "DELETE FROM sync_jobs WHERE id = :id"
            ), {"id": job.id})
            session.commit()

    def _handle_failure(self, job: SyncJob, error: str):
        logger.error("Job %s failed: %s", job.id, error)
        job.attempts += 1
        job.last_error = error

        with get_session() as session:
            if job.attempts >= job.max_attempts:
                session.execute(text("""
                    UPDATE sync_jobs
                    SET status = 'dead', last_error = :err, attempts = :attempts
                    WHERE id = :id
                """), {"err": error, "attempts": job.attempts, "id": job.id})
                logger.warning("Job %s marked dead after %d attempts", job.id, job.attempts)
            else:
                backoff = _exponential_backoff(job.attempts)
                session.execute(text("""
                    UPDATE sync_jobs
                    SET status = 'pending',
                        attempts = :attempts,
                        last_error = :err,
                        scheduled_at = now() + :backoff * interval '1 second'
                    WHERE id = :id
                """), {
                    "attempts": job.attempts,
                    "err": error,
                    "backoff": backoff,
                    "id": job.id,
                })
            session.commit()


# ── Builder ───────────────────────────────────────────────────────────────────

class JobQueueBuilder:
    def __init__(self):
        self._syncers: list[SourceSyncer] = []

    def add(self, syncer: SourceSyncer) -> JobQueueBuilder:
        self._syncers.append(syncer)
        return self

    def build(self) -> JobQueue:
        syncers = {s.source_type: s for s in self._syncers}
        router = SyncRouter(syncers)
        return JobQueue(router)


# ── Helpers ───────────────────────────────────────────────────────────────────

def _enqueue(session: Session, job: SyncJob):
    session.execute(text("""
        INSERT INTO sync_jobs
            (id, source_id, snapshot_id, source_type, job_type, payload,
             priority, scheduled_at, attempts, max_attempts, status)
        VALUES
            (gen_random_uuid(), :source_id, :snapshot_id, :source_type, :job_type,
             CAST(:payload AS jsonb), :priority, :scheduled_at, :attempts, :max_attempts, 'pending')
    """), {
        "source_id":    job.source_id,
        "snapshot_id":  job.snapshot_id,
        "source_type":  job.source_type,
        "job_type":     job.job_type,
        "payload":      _json(job.payload),
        "priority":     job.priority,
        "scheduled_at": job.scheduled_at,
        "attempts":     job.attempts,
        "max_attempts": job.max_attempts,
    })


def _upsert_artifact(session: Session, job: SyncJob, raw: RawArtifact):
    """
    Insert new artifact or supersede if external_id already exists with changed content.
    Resolves parent_external_id → parent_id before inserting.
    """
    # Resolve parent artifact id from parent_external_id
    parent_id = None
    if raw.parent_external_id:
        row = session.execute(text("""
            SELECT id FROM artifacts
            WHERE source_id = :source_id
              AND external_id = :external_id
              AND status != 'superseded'
            ORDER BY version DESC
            LIMIT 1
        """), {"source_id": job.source_id, "external_id": raw.parent_external_id}).fetchone()
        if row:
            parent_id = row.id

    existing = session.execute(text("""
        SELECT id, content FROM artifacts
        WHERE source_id = :source_id
          AND external_id = :external_id
          AND status != 'superseded'
        ORDER BY version DESC
        LIMIT 1
    """), {"source_id": job.source_id, "external_id": raw.external_id}).fetchone()

    if existing:
        if existing.content == raw.content:
            return  # unchanged — skip
        old_version = session.execute(text(
            "SELECT version FROM artifacts WHERE id = :id"
        ), {"id": existing.id}).scalar() or 1

        new_id = f"artifact-{uuid.uuid4().hex}"
        session.execute(text("""
            UPDATE artifacts SET status = 'superseded', superseded_by = :new_id
            WHERE id = :old_id
        """), {"new_id": new_id, "old_id": existing.id})

        _insert_artifact(session, new_id, job, raw, version=old_version + 1, parent_id=parent_id)
    else:
        new_id = f"artifact-{uuid.uuid4().hex}"
        _insert_artifact(session, new_id, job, raw, version=1, parent_id=parent_id)


def _insert_artifact(session: Session, artifact_id: str, job: SyncJob, raw: RawArtifact,
                     version: int, parent_id: str | None = None):
    import json
    session.execute(text("""
        INSERT INTO artifacts
            (id, source_id, snapshot_id, external_id, version,
             type, label, content, source_url, parent_id, metadata, status, created_at)
        VALUES
            (:id, :source_id, :snapshot_id, :external_id, :version,
             :type, :label, :content, :source_url, :parent_id,
             CAST(:metadata AS jsonb), 'pending', now())
    """), {
        "id":          artifact_id,
        "source_id":   job.source_id,
        "snapshot_id": job.snapshot_id,
        "external_id": raw.external_id,
        "version":     version,
        "type":        raw.type,
        "label":       raw.label,
        "content":     raw.content,
        "source_url":  raw.source_url,
        "parent_id":   parent_id,
        "metadata":    json.dumps(raw.metadata),
    })


def _on_job_complete(session: Session, job: SyncJob, is_terminal: bool):
    """
    Track snapshot progress. Only mark complete when the job chain is fully done
    (no next_job pagination). This handles dynamic pagination depth correctly.
    """
    if not job.snapshot_id or not is_terminal:
        return

    result = session.execute(text("""
        UPDATE snapshots
        SET status = 'complete',
            completed_jobs = completed_jobs + 1
        WHERE id = :snapshot_id
          AND status = 'processing'
        RETURNING id, source_id,
                  (SELECT kg_id FROM sources WHERE id = source_id) as kg_id
    """), {"snapshot_id": job.snapshot_id}).fetchone()

    if result:
        _enqueue(session, SyncJob(
            source_id=str(result.source_id),
            source_type="insight",
            job_type="insight_batch",
            snapshot_id=job.snapshot_id,
            payload={"snapshot_id": job.snapshot_id, "kg_id": str(result.kg_id)},
            priority=5,
        ))
        logger.info("Snapshot %s complete — insight batch enqueued", job.snapshot_id)


def _row_to_job(row) -> SyncJob:
    return SyncJob(
        id=str(row.id),
        source_id=str(row.source_id),
        snapshot_id=str(row.snapshot_id) if row.snapshot_id else None,
        source_type=row.source_type,
        job_type=row.job_type,
        payload=dict(row.payload) if row.payload else {},
        priority=row.priority,
        scheduled_at=row.scheduled_at,
        attempts=row.attempts,
        max_attempts=row.max_attempts,
        last_error=row.last_error,
    )


def _exponential_backoff(attempts: int) -> int:
    """Returns backoff in seconds: 30s, 120s, 600s, 3600s max."""
    return min(30 * (4 ** (attempts - 1)), 3600)


def _sync_interval(source: Source) -> timedelta:
    intervals = {
        "reddit":  timedelta(hours=1),
        "bluesky": timedelta(minutes=30),
        "email":   timedelta(minutes=15),
    }
    return intervals.get(source.type, timedelta(hours=1))


def _json(d: dict) -> str:
    import json
    return json.dumps(d)
