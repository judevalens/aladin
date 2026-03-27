from __future__ import annotations
import logging
import time
from datetime import datetime, timezone, timedelta

import httpx

from ..base import SourceSyncer
from ..domain import SyncJob, SyncResult, RawArtifact
from ...db.models import Source, Snapshot

logger = logging.getLogger(__name__)

REDDIT_API = "https://www.reddit.com"
USER_AGENT = "aladin-bot/0.1"
RATE_LIMIT_RPM = 10  # conservative unauthenticated limit — bump to 55 once OAuth is set up


class RedditSyncer(SourceSyncer):
    source_type = "reddit"

    def __init__(self):
        self._last_request_at: float = 0.0
        self._min_interval: float = 60.0 / RATE_LIMIT_RPM

    # ── SourceSyncer interface ────────────────────────────────────────────────

    def plan(self, source: Source, snapshot: Snapshot) -> list[SyncJob]:
        """
        Returns jobs for one sync cycle:
        1. One ensure_feed job to upsert the top-level feed artifact
        2. One fetch_listing job to pick up new posts
        3. fetch_thread jobs for still-active threads (score-bracket based)
        """
        config = source.config
        jobs: list[SyncJob] = []

        # Ensure top-level feed artifact exists
        jobs.append(SyncJob(
            source_id=str(source.id),
            source_type=self.source_type,
            job_type="ensure_feed",
            payload={"subreddit": config.get("subreddit")},
            priority=10,  # runs first
        ))

        # New posts listing
        jobs.append(SyncJob(
            source_id=str(source.id),
            source_type=self.source_type,
            job_type="fetch_listing",
            payload={
                "subreddit":   config.get("subreddit"),
                "cursor":      config.get("after"),
                "stop_at_id":  config.get("last_seen_id"),
                "min_score":   config.get("min_score", 10),
            },
            priority=1,
        ))

        # Re-fetch active threads
        for job in self._plan_refetch_jobs(source):
            jobs.append(job)

        return jobs

    def execute(self, job: SyncJob) -> SyncResult:
        if job.job_type == "ensure_feed":
            return self._ensure_feed(job)
        if job.job_type == "fetch_listing":
            return self._fetch_listing(job)
        if job.job_type == "fetch_thread":
            return self._fetch_thread(job)
        raise ValueError(f"RedditSyncer: unknown job_type '{job.job_type}'")

    # ── Feed artifact ────────────────────────────────────────────────────────

    def _ensure_feed(self, job: SyncJob) -> SyncResult:
        """
        Upserts the top-level feed artifact for this subreddit.
        This is the artifact that shows in the dashboard grid.
        Posts are children of this artifact.
        No HTTP request needed — just a DB upsert.
        """
        subreddit = job.payload["subreddit"]
        feed_artifact = RawArtifact(
            external_id=f"feed:reddit:{subreddit}",
            type="feed",
            label=f"r/{subreddit}",
            content=f"Live feed from r/{subreddit}",
            source_url=f"https://reddit.com/r/{subreddit}",
            metadata={
                "platform":  "reddit",
                "subreddit": subreddit,
                "feed_type": "subreddit",
            },
        )
        return SyncResult(artifacts=[feed_artifact])

    # ── Listing ───────────────────────────────────────────────────────────────

    def _fetch_listing(self, job: SyncJob) -> SyncResult:
        p = job.payload
        subreddit = p["subreddit"]
        cursor    = p.get("cursor")
        stop_at   = p.get("stop_at_id")
        min_score = p.get("min_score", 10)

        params = {"limit": 100, "raw_json": 1}
        if cursor:
            params["after"] = cursor

        try:
            data = self._get(f"/r/{subreddit}/new.json", params)
        except httpx.HTTPStatusError as exc:
            return self._handle_http_error(exc, job)

        posts = data["data"]["children"]
        next_cursor = data["data"].get("after")
        artifacts: list[RawArtifact] = []
        hit_stop = False

        for post in posts:
            pd = post["data"]
            fullname = pd["name"]  # e.g. t3_abc123

            if stop_at and fullname == stop_at:
                hit_stop = True
                break

            if pd.get("score", 0) < min_score:
                continue

            if pd.get("removed_by_category"):
                continue

            artifacts.append(_post_to_artifact(pd, subreddit))

        # Determine follow-up
        next_job = None
        if next_cursor and not hit_stop and posts:
            next_job = SyncJob(
                source_id=job.source_id,
                source_type=self.source_type,
                job_type="fetch_listing",
                payload={**p, "cursor": next_cursor},
                priority=1,
            )

        return SyncResult(artifacts=artifacts, next_job=next_job)

    # ── Thread re-fetch ───────────────────────────────────────────────────────

    def _fetch_thread(self, job: SyncJob) -> SyncResult:
        p = job.payload
        post_id   = p["post_id"]      # e.g. t3_abc123 — strip t3_ for URL
        subreddit = p["subreddit"]
        short_id  = post_id.removeprefix("t3_")

        try:
            data = self._get(f"/r/{subreddit}/comments/{short_id}.json", {"raw_json": 1})
        except httpx.HTTPStatusError as exc:
            if exc.response.status_code == 404:
                # Post deleted — return empty, no re-fetch
                logger.info("Post %s not found (deleted), skipping re-fetch", post_id)
                return SyncResult()
            return self._handle_http_error(exc, job)

        post_data = data[0]["data"]["children"][0]["data"]
        artifact = _post_to_artifact(post_data, subreddit)

        # Schedule next re-fetch based on current score + age
        next_job = _schedule_refetch(job, post_data)

        return SyncResult(artifacts=[artifact], next_job=next_job)

    # ── Active thread planning ────────────────────────────────────────────────

    def _plan_refetch_jobs(self, source: Source) -> list[SyncJob]:
        """
        Query DB for artifacts from this source that are still within their
        re-fetch window based on initial score and age.
        """
        from sqlalchemy import text
        from ...db import get_session

        jobs = []
        with get_session() as session:
            rows = session.execute(text("""
                SELECT
                    id,
                    external_id,
                    (metadata->>'score')::int         AS score,
                    (metadata->>'subreddit')::text     AS subreddit,
                    created_at
                FROM artifacts
                WHERE source_id = :source_id
                  AND status NOT IN ('superseded', 'dismissed')
                  AND external_id LIKE 't3_%'
                  AND created_at > now() - interval '48 hours'
            """), {"source_id": str(source.id)}).fetchall()

            now = datetime.now(timezone.utc)
            for row in rows:
                age_hours = (now - row.created_at.replace(tzinfo=timezone.utc)).total_seconds() / 3600
                next_at = _next_refetch_at(row.score or 0, age_hours)
                if next_at is None:
                    continue
                jobs.append(SyncJob(
                    source_id=str(source.id),
                    source_type=self.source_type,
                    job_type="fetch_thread",
                    payload={
                        "post_id":   row.external_id,
                        "subreddit": row.subreddit or source.config.get("subreddit"),
                        "score_at_last_fetch": row.score or 0,
                    },
                    priority=0,
                    scheduled_at=next_at,
                ))

        return jobs

    # ── HTTP ──────────────────────────────────────────────────────────────────

    def _get(self, path: str, params: dict) -> dict:
        self._wait_for_token()
        resp = httpx.get(
            f"{REDDIT_API}{path}",
            params=params,
            headers={"User-Agent": USER_AGENT},
            timeout=15,
        )
        resp.raise_for_status()
        return resp.json()

    def _wait_for_token(self):
        elapsed = time.monotonic() - self._last_request_at
        gap = self._min_interval - elapsed
        if gap > 0:
            time.sleep(gap)
        self._last_request_at = time.monotonic()

    def _handle_http_error(self, exc: httpx.HTTPStatusError, job: SyncJob) -> SyncResult:
        status = exc.response.status_code
        if status == 429:
            logger.warning("Reddit rate limited — requeueing job %s", job.id)
        else:
            logger.error("Reddit HTTP %d — requeueing job %s", status, job.id)

        requeue = SyncJob(
            source_id=job.source_id,
            source_type=job.source_type,
            job_type=job.job_type,
            payload=job.payload,
            priority=job.priority,
            attempts=job.attempts + 1,
            max_attempts=job.max_attempts,
            last_error=str(exc),
            scheduled_at=datetime.now(timezone.utc) + _backoff(job.attempts + 1),
        )
        return SyncResult(requeue_job=requeue)


# ── Helpers ───────────────────────────────────────────────────────────────────

def _post_to_artifact(pd: dict, subreddit: str) -> RawArtifact:
    title    = pd.get("title", "")
    selftext = pd.get("selftext", "")
    content  = f"{title}\n\n{selftext}".strip() if selftext else title

    return RawArtifact(
        external_id=pd["name"],   # t3_abc123
        type="post",
        label=title[:120],
        content=content,
        source_url=f"https://reddit.com{pd.get('permalink', '')}",
        parent_external_id=f"feed:reddit:{subreddit}",  # child of feed artifact
        metadata={
            "reddit_id":    pd["name"],
            "subreddit":    subreddit,
            "author":       pd.get("author", ""),
            "score":        pd.get("score", 0),
            "url":          pd.get("url", ""),
            "flair":        pd.get("link_flair_text"),
            "created_utc":  pd.get("created_utc"),
            "num_comments": pd.get("num_comments", 0),
            "edited":       pd.get("edited", False),
        },
    )


def _next_refetch_at(score: int, age_hours: float) -> datetime | None:
    now = datetime.now(timezone.utc)
    if score >= 500 and age_hours < 24:
        return now + timedelta(minutes=30)
    if score >= 100 and age_hours < 12:
        return now + timedelta(hours=1)
    if score >= 20 and age_hours < 6:
        return now + timedelta(hours=2)
    return None


def _schedule_refetch(job: SyncJob, post_data: dict) -> SyncJob | None:
    score     = post_data.get("score", 0)
    created   = post_data.get("created_utc", 0)
    age_hours = (time.time() - created) / 3600
    next_at   = _next_refetch_at(score, age_hours)

    if next_at is None:
        return None

    return SyncJob(
        source_id=job.source_id,
        source_type=job.source_type,
        job_type="fetch_thread",
        payload={**job.payload, "score_at_last_fetch": score},
        priority=0,
        scheduled_at=next_at,
    )


def _backoff(attempts: int) -> timedelta:
    seconds = min(30 * (4 ** (attempts - 1)), 3600)
    return timedelta(seconds=seconds)
