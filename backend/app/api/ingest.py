from flask import Blueprint, jsonify, request
import httpx

from app.ingest.url import UrlIngestService
from app.ingest.url.base import Post

bp = Blueprint("ingest", __name__)
_service = UrlIngestService()


def _serialize_post(post: Post) -> dict:
    return {
        "platform": post.platform,
        "postId": post.post_id,
        "url": post.url,
        "authorHandle": post.author_handle,
        "authorName": post.author_name,
        "content": post.content,
        "createdAt": post.created_at.isoformat(),
        "likeCount": post.like_count,
        "repostCount": post.repost_count,
        "replyCount": post.reply_count,
        "embeds": post.embeds,
        "thread": [_serialize_post(p) for p in post.thread],
    }


@bp.post("/url")
def ingest_url():
    data = request.get_json()
    url = (data or {}).get("url", "").strip()

    if not url:
        return jsonify({"error": "url is required"}), 400

    if not _service.supports(url):
        return jsonify({"error": "Unsupported URL", "url": url}), 422

    try:
        result = _service.ingest(url)
    except httpx.HTTPStatusError as e:
        return jsonify({"error": f"Failed to fetch content: {e.response.status_code}"}), 502
    except httpx.RequestError:
        return jsonify({"error": "Network error fetching URL"}), 502
    except Exception as e:
        return jsonify({"error": str(e)}), 500

    return jsonify({
        "type": result.type,
        "rawUrl": result.raw_url,
        "post": _serialize_post(result.post) if result.post else None,
    })
