import re
from datetime import datetime, timezone

import httpx

from app.ingest.url.base import IngestStrategy, IngestedContent, Post

BSKY_API = "https://public.api.bsky.app/xrpc"

# https://bsky.app/profile/<handle>/post/<rkey>
_URL_RE = re.compile(
    r"https?://bsky\.app/profile/(?P<handle>[^/]+)/post/(?P<rkey>[^/?#]+)"
)


class BlueSkyStrategy(IngestStrategy):

    def can_handle(self, url: str) -> bool:
        return bool(_URL_RE.match(url))

    def ingest(self, url: str) -> IngestedContent:
        m = _URL_RE.match(url)
        if not m:
            raise ValueError(f"Not a Bluesky post URL: {url}")

        handle = m.group("handle")
        rkey = m.group("rkey")

        did = self._resolve_handle(handle)
        at_uri = f"at://{did}/app.bsky.feed.post/{rkey}"
        thread_data = self._fetch_thread(at_uri)

        root_post = self._parse_post(thread_data["thread"]["post"], url)

        # Collect parent context (walk up the thread)
        parents: list[Post] = []
        cursor = thread_data["thread"].get("parent")
        while cursor and cursor.get("post"):
            parents.insert(0, self._parse_post(cursor["post"]))
            cursor = cursor.get("parent")

        root_post.thread = parents

        return IngestedContent(type="post", raw_url=url, post=root_post)

    # ── Private helpers ──────────────────────────────────────────────────────

    def _resolve_handle(self, handle: str) -> str:
        resp = httpx.get(
            f"{BSKY_API}/com.atproto.identity.resolveHandle",
            params={"handle": handle},
            timeout=10,
        )
        resp.raise_for_status()
        return resp.json()["did"]

    def _fetch_thread(self, at_uri: str) -> dict:
        resp = httpx.get(
            f"{BSKY_API}/app.bsky.feed.getPostThread",
            params={"uri": at_uri, "depth": 0, "parentHeight": 5},
            timeout=10,
        )
        resp.raise_for_status()
        return resp.json()

    def _parse_post(self, post_data: dict, url: str = "") -> Post:
        author = post_data.get("author", {})
        record = post_data.get("record", {})

        created_at_raw = record.get("createdAt", "")
        try:
            created_at = datetime.fromisoformat(
                created_at_raw.replace("Z", "+00:00")
            )
        except ValueError:
            created_at = datetime.now(timezone.utc)

        embeds = self._parse_embeds(post_data.get("embed"))

        return Post(
            platform="bluesky",
            post_id=post_data.get("uri", ""),
            url=url,
            author_handle=author.get("handle", ""),
            author_name=author.get("displayName") or author.get("handle", ""),
            content=record.get("text", ""),
            created_at=created_at,
            like_count=post_data.get("likeCount", 0),
            repost_count=post_data.get("repostCount", 0),
            reply_count=post_data.get("replyCount", 0),
            embeds=embeds,
        )

    def _parse_embeds(self, embed: dict | None) -> list[dict]:
        if not embed:
            return []

        embed_type = embed.get("$type", "")

        if "external" in embed_type:
            ext = embed.get("external", {})
            return [{"type": "link", "url": ext.get("uri"), "title": ext.get("title"), "description": ext.get("description")}]

        if "images" in embed_type:
            return [{"type": "image", "alt": img.get("alt", "")} for img in embed.get("images", [])]

        if "record" in embed_type:
            rec = embed.get("record", {})
            return [{"type": "quote", "uri": rec.get("uri", "")}]

        return []
