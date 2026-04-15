from __future__ import annotations
import uuid

from flask import Blueprint, jsonify, request

from app.db import get_session
from app.db.models import KnowledgeGraph, Source
from app.db.seed import DEFAULT_USER_ID

bp = Blueprint("sources", __name__)


SOURCE_KINDS = {
    "reddit_subreddit",
    "bluesky_search",
    "bluesky_account",
    "twitter_search",
}


def _serialize(source: Source) -> dict:
    return {
        "id": str(source.id),
        "name": source.name,
        "type": source.type,
        "syncMode": source.sync_mode,
        "syncState": source.sync_state,
        "config": source.config or {},
        "autoPromoteThreshold": source.auto_promote_threshold,
        "suggestThreshold": source.suggest_threshold,
        "createdAt": source.created_at.isoformat(),
        "lastSyncedAt": source.last_synced_at.isoformat() if source.last_synced_at else None,
    }


def _default_kg(session) -> KnowledgeGraph:
    kg = (
        session.query(KnowledgeGraph)
        .filter(KnowledgeGraph.user_id == DEFAULT_USER_ID)
        .order_by(KnowledgeGraph.created_at.asc())
        .first()
    )
    if kg:
        return kg

    kg = KnowledgeGraph(
        user_id=DEFAULT_USER_ID,
        name="Default Research Graph",
        description="Default workspace for persisted feed sources.",
    )
    session.add(kg)
    session.flush()
    return kg


def _coerce_bool(value: object) -> bool:
    if isinstance(value, bool):
        return value
    if isinstance(value, str):
        return value.strip().lower() in {"1", "true", "yes", "on"}
    return bool(value)


def _clean_str(value: object, field: str) -> str:
    cleaned = str(value or "").strip()
    if not cleaned:
        raise ValueError(f"{field} is required")
    return cleaned


def _build_source_payload(data: dict) -> dict:
    kind = _clean_str(data.get("kind"), "kind")
    if kind not in SOURCE_KINDS:
        raise ValueError(f"Unsupported source kind '{kind}'")

    name = str(data.get("name") or "").strip()

    if kind == "reddit_subreddit":
        subreddit = _clean_str(data.get("subreddit"), "subreddit").removeprefix("r/").strip("/")
        min_score = max(int(data.get("minScore", 10)), 0)
        include_comments = _coerce_bool(data.get("includeComments", True))
        top_comments_n = max(int(data.get("topComments", 5)), 0)
        sort = str(data.get("sort") or "new").strip().lower()
        if sort not in {"new", "hot", "top"}:
            raise ValueError("sort must be one of: new, hot, top")
        return {
            "name": name or f"r/{subreddit}",
            "type": "reddit",
            "sync_mode": "poll",
            "config": {
                "subreddit": subreddit,
                "min_score": min_score,
                "include_comments": include_comments,
                "top_comments_n": top_comments_n,
                "sort": sort,
                "after": None,
                "last_seen_id": None,
            },
        }

    if kind == "bluesky_search":
        query = _clean_str(data.get("query"), "query")
        return {
            "name": name or f"Bluesky: {query[:48]}",
            "type": "bluesky",
            "sync_mode": "poll",
            "config": {
                "mode": "search",
                "query": query,
                "cursor": None,
                "limit": max(int(data.get("limit", 50)), 1),
            },
        }

    if kind == "bluesky_account":
        handle = _clean_str(data.get("handle"), "handle").lstrip("@")
        return {
            "name": name or f"@{handle}",
            "type": "bluesky",
            "sync_mode": "poll",
            "config": {
                "mode": "account",
                "handle": handle,
                "cursor": None,
                "limit": max(int(data.get("limit", 50)), 1),
            },
        }

    query = _clean_str(data.get("query"), "query")
    return {
        "name": name or f"X: {query[:48]}",
        "type": "twitter",
        "sync_mode": "poll",
        "config": {
            "mode": "search",
            "query": query,
            "since_id": None,
            "min_likes": max(int(data.get("minLikes", 5)), 0),
            "max_results": max(int(data.get("maxResults", 25)), 10),
        },
    }


@bp.get("/")
def list_sources():
    with get_session() as session:
        rows = (
            session.query(Source)
            .filter(Source.user_id == DEFAULT_USER_ID)
            .order_by(Source.created_at.desc())
            .all()
        )
        return jsonify([_serialize(source) for source in rows])


@bp.post("/")
def create_source():
    data = request.get_json() or {}

    try:
        payload = _build_source_payload(data)
    except (TypeError, ValueError) as exc:
        return jsonify({"error": str(exc)}), 400

    with get_session() as session:
        kg = _default_kg(session)
        source = Source(
            user_id=DEFAULT_USER_ID,
            kg_id=kg.id,
            name=payload["name"],
            type=payload["type"],
            sync_mode=payload["sync_mode"],
            sync_state="active",
            config=payload["config"],
        )
        session.add(source)
        session.commit()
        session.refresh(source)
        return jsonify(_serialize(source)), 201


@bp.delete("/<source_id>")
def delete_source(source_id: str):
    try:
        source_uuid = uuid.UUID(source_id)
    except ValueError:
        return jsonify({"error": "Invalid source id"}), 400

    with get_session() as session:
        source = (
            session.query(Source)
            .filter(Source.id == source_uuid, Source.user_id == DEFAULT_USER_ID)
            .first()
        )
        if not source:
            return jsonify({"error": "Source not found"}), 404
        session.delete(source)
        session.commit()
        return jsonify({"ok": True})
