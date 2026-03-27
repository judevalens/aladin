from __future__ import annotations
import logging
import os
import time
from datetime import datetime, timezone, timedelta

import httpx

from ..base import SourceSyncer
from ..domain import SyncJob, SyncResult, RawArtifact
from ...db.models import Source, Snapshot

logger = logging.getLogger(__name__)

TWITTER_API = "https://api.twitter.com/2"

# Basic tier: 10 req/15min per app = ~1 req/90s
# We use 100s to be safe. Bump this down if you're on a Pro/Enterprise plan.
REQUEST_INTERVAL = 100  # seconds

# Fields requested from Twitter API v2
TWEET_FIELDS = "id,text,author_id,created_at,public_metrics,conversation_id,referenced_tweets,entities,note_tweet"
EXPANSIONS   = "author_id,referenced_tweets.id"
USER_FIELDS  = "id,name,username,profile_image_url"


class TwitterSyncer(SourceSyncer):
    source_type = "twitter"

    def __init__(self):
        self._last_request_at: float = 0.0
        self._bearer_token: str | None = os.getenv("TWITTER_BEARER_TOKEN")

    # ── SourceSyncer interface ────────────────────────────────────────────────

    def plan(self, source: Source, snapshot: Snapshot) -> list[SyncJob]:
        """
        Two modes driven by source.config:
          - mode="user"   → follow a Twitter account
          - mode="search" → track a keyword / hashtag

        Jobs emitted per cycle:
          1. ensure_feed (priority 10) — upsert the top-level feed artifact
          2. fetch_timeline or fetch_search (priority 1) — new tweets
        """
        config = source.config
        jobs: list[SyncJob] = []

        jobs.append(SyncJob(
            source_id=str(source.id),
            source_type=self.source_type,
            job_type="ensure_feed",
            payload={"mode": config.get("mode", "user"), "config": config},
            priority=10,
        ))

        mode = config.get("mode", "user")
        if mode == "user":
            jobs.append(SyncJob(
                source_id=str(source.id),
                source_type=self.source_type,
                job_type="fetch_timeline",
                payload={
                    "username":       config.get("username"),
                    "since_id":       config.get("since_id"),
                    "min_likes":      config.get("min_likes", 5),
                    "max_results":    min(config.get("max_results", 100), 100),
                },
                priority=1,
            ))
        elif mode == "search":
            jobs.append(SyncJob(
                source_id=str(source.id),
                source_type=self.source_type,
                job_type="fetch_search",
                payload={
                    "query":       config.get("query"),
                    "since_id":    config.get("since_id"),
                    "min_likes":   config.get("min_likes", 10),
                    "max_results": min(config.get("max_results", 100), 100),
                },
                priority=1,
            ))

        return jobs

    def execute(self, job: SyncJob) -> SyncResult:
        if job.job_type == "ensure_feed":
            return self._ensure_feed(job)
        if job.job_type == "fetch_timeline":
            return self._fetch_timeline(job)
        if job.job_type == "fetch_search":
            return self._fetch_search(job)
        raise ValueError(f"TwitterSyncer: unknown job_type '{job.job_type}'")

    # ── Feed artifact ────────────────────────────────────────────────────────

    def _ensure_feed(self, job: SyncJob) -> SyncResult:
        config = job.payload["config"]
        mode   = job.payload["mode"]

        if mode == "user":
            username = config.get("username", "unknown")
            feed = RawArtifact(
                external_id=f"feed:twitter:{username}",
                type="feed",
                label=f"@{username}",
                content=f"Live tweets from @{username}",
                source_url=f"https://x.com/{username}",
                metadata={
                    "platform":  "twitter",
                    "mode":      "user",
                    "username":  username,
                },
            )
        else:
            query = config.get("query", "")
            slug  = query[:40].replace(" ", "_").lower()
            feed = RawArtifact(
                external_id=f"feed:twitter:search:{slug}",
                type="feed",
                label=f"X: {query[:60]}",
                content=f"Live X search: {query}",
                source_url=f"https://x.com/search?q={query}&src=typed_query",
                metadata={
                    "platform": "twitter",
                    "mode":     "search",
                    "query":    query,
                },
            )

        return SyncResult(artifacts=[feed])

    # ── User timeline ────────────────────────────────────────────────────────

    def _fetch_timeline(self, job: SyncJob) -> SyncResult:
        p        = job.payload
        username = p["username"]
        since_id = p.get("since_id")
        min_likes = p.get("min_likes", 5)
        max_results = p.get("max_results", 100)

        # Resolve username → user_id (cached in payload after first call)
        user_id = p.get("user_id")
        if not user_id:
            try:
                user_id = self._resolve_user_id(username)
            except httpx.HTTPStatusError as exc:
                return self._handle_http_error(exc, job)
            if not user_id:
                logger.warning("TwitterSyncer: could not resolve user_id for @%s", username)
                return SyncResult()

        params = {
            "max_results":  min(max_results, 100),
            "tweet.fields": TWEET_FIELDS,
            "expansions":   EXPANSIONS,
            "user.fields":  USER_FIELDS,
            "exclude":      "replies,retweets",  # top-level tweets only
        }
        if since_id:
            params["since_id"] = since_id

        # If paginating forward (next_token from a previous page)
        next_token = p.get("next_token")
        if next_token:
            params["pagination_token"] = next_token

        try:
            data = self._get(f"/users/{user_id}/tweets", params)
        except httpx.HTTPStatusError as exc:
            return self._handle_http_error(exc, job)

        tweets    = data.get("data", [])
        users_map = _build_users_map(data)
        feed_eid  = f"feed:twitter:{username}"

        artifacts = [
            _tweet_to_artifact(t, users_map, feed_eid)
            for t in tweets
            if t.get("public_metrics", {}).get("like_count", 0) >= min_likes
        ]

        # Pagination — newest first, so stop when we hit since_id
        meta       = data.get("meta", {})
        page_token = meta.get("next_token")
        next_job   = None

        if page_token and tweets:
            next_job = SyncJob(
                source_id=job.source_id,
                source_type=self.source_type,
                job_type="fetch_timeline",
                payload={**p, "user_id": user_id, "next_token": page_token},
                priority=1,
            )

        return SyncResult(artifacts=artifacts, next_job=next_job)

    # ── Search ───────────────────────────────────────────────────────────────

    def _fetch_search(self, job: SyncJob) -> SyncResult:
        p         = job.payload
        query     = p["query"]
        since_id  = p.get("since_id")
        min_likes = p.get("min_likes", 10)
        max_results = p.get("max_results", 100)

        # Build search query — exclude replies and retweets unless explicitly included
        full_query = query
        if "-is:reply"   not in query: full_query += " -is:reply"
        if "-is:retweet" not in query: full_query += " -is:retweet"

        params = {
            "query":        full_query,
            "max_results":  min(max_results, 100),
            "tweet.fields": TWEET_FIELDS,
            "expansions":   EXPANSIONS,
            "user.fields":  USER_FIELDS,
            "sort_order":   "recency",
        }
        if since_id:
            params["since_id"] = since_id

        next_token = p.get("next_token")
        if next_token:
            params["next_token"] = next_token

        try:
            data = self._get("/tweets/search/recent", params)
        except httpx.HTTPStatusError as exc:
            return self._handle_http_error(exc, job)

        tweets    = data.get("data", [])
        users_map = _build_users_map(data)
        slug      = query[:40].replace(" ", "_").lower()
        feed_eid  = f"feed:twitter:search:{slug}"

        artifacts = [
            _tweet_to_artifact(t, users_map, feed_eid)
            for t in tweets
            if t.get("public_metrics", {}).get("like_count", 0) >= min_likes
        ]

        meta       = data.get("meta", {})
        page_token = meta.get("next_token")
        next_job   = None

        if page_token and tweets:
            next_job = SyncJob(
                source_id=job.source_id,
                source_type=self.source_type,
                job_type="fetch_search",
                payload={**p, "next_token": page_token},
                priority=1,
            )

        return SyncResult(artifacts=artifacts, next_job=next_job)

    # ── HTTP ──────────────────────────────────────────────────────────────────

    def _resolve_user_id(self, username: str) -> str | None:
        """Look up numeric user ID from username. Consumes one rate limit slot."""
        self._wait_for_token()
        resp = httpx.get(
            f"{TWITTER_API}/users/by/username/{username}",
            params={"user.fields": "id,name,username"},
            headers=self._auth_headers(),
            timeout=15,
        )
        resp.raise_for_status()
        data = resp.json()
        return data.get("data", {}).get("id")

    def _get(self, path: str, params: dict) -> dict:
        self._wait_for_token()
        resp = httpx.get(
            f"{TWITTER_API}{path}",
            params=params,
            headers=self._auth_headers(),
            timeout=15,
        )
        resp.raise_for_status()
        return resp.json()

    def _auth_headers(self) -> dict:
        if not self._bearer_token:
            raise RuntimeError(
                "TWITTER_BEARER_TOKEN not set. "
                "Create a project at developer.twitter.com and add the Bearer Token to .env."
            )
        return {"Authorization": f"Bearer {self._bearer_token}"}

    def _wait_for_token(self):
        elapsed = time.monotonic() - self._last_request_at
        gap = REQUEST_INTERVAL - elapsed
        if gap > 0:
            logger.debug("TwitterSyncer: rate limit — sleeping %.1fs", gap)
            time.sleep(gap)
        self._last_request_at = time.monotonic()

    def _handle_http_error(self, exc: httpx.HTTPStatusError, job: SyncJob) -> SyncResult:
        status = exc.response.status_code
        if status == 429:
            logger.warning("Twitter rate limited — requeueing job %s", job.id)
        elif status == 401:
            logger.error("Twitter 401 Unauthorized — check TWITTER_BEARER_TOKEN")
            return SyncResult()  # don't retry auth failures
        else:
            logger.error("Twitter HTTP %d — requeueing job %s", status, job.id)

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

def _build_users_map(data: dict) -> dict[str, dict]:
    """Build author_id → user dict from API response includes."""
    users = data.get("includes", {}).get("users", [])
    return {u["id"]: u for u in users}


def _tweet_to_artifact(tweet: dict, users_map: dict, feed_external_id: str) -> RawArtifact:
    tweet_id = tweet["id"]
    text     = tweet.get("text", "")

    # Long tweets have their full text in note_tweet
    if tweet.get("note_tweet"):
        text = tweet["note_tweet"].get("text", text)

    metrics  = tweet.get("public_metrics", {})
    author   = users_map.get(tweet.get("author_id", ""), {})
    username = author.get("username", "unknown")
    name     = author.get("name", username)
    created  = tweet.get("created_at", "")

    label = text[:120].replace("\n", " ")

    return RawArtifact(
        external_id=f"tweet:{tweet_id}",
        type="post",
        label=label,
        content=text,
        source_url=f"https://x.com/{username}/status/{tweet_id}",
        parent_external_id=feed_external_id,
        metadata={
            "tweet_id":       tweet_id,
            "author_id":      tweet.get("author_id"),
            "author":         username,
            "author_name":    name,
            "score":          metrics.get("like_count", 0),   # use likes as score (same field as reddit)
            "like_count":     metrics.get("like_count", 0),
            "retweet_count":  metrics.get("retweet_count", 0),
            "reply_count":    metrics.get("reply_count", 0),
            "quote_count":    metrics.get("quote_count", 0),
            "num_comments":   metrics.get("reply_count", 0),  # alias for UI consistency
            "conversation_id": tweet.get("conversation_id"),
            "created_at":     created,
            "platform":       "twitter",
        },
    )


def _backoff(attempts: int) -> timedelta:
    seconds = min(60 * (3 ** (attempts - 1)), 3600)
    return timedelta(seconds=seconds)
