"""
Unified feed API — all artifact leaf nodes across all sources,
ranked by signal (engagement + recency).

GET  /api/feed
POST /api/feed/<id>/save
POST /api/feed/<id>/dismiss
POST /api/feed/<id>/unsave
"""
from flask import Blueprint, jsonify, request
from sqlalchemy import text
from app.db import get_session

bp = Blueprint("feed", __name__)


def _serialize(row) -> dict:
    return {
        "id":           row.id,
        "type":         row.type,
        "label":        row.label,
        "content":      row.content[:500] if row.content else "",
        "sourceUrl":    row.source_url,
        "enrichment":   row.enrichment,
        "metadata":     row.metadata or {},
        "userStatus":   row.user_status,
        "sourceType":   row.source_type,
        "sourceName":   row.source_name,
        "signalScore":  float(row.signal_score) if row.signal_score is not None else 0.0,
        "createdAt":    row.created_at.isoformat(),
    }


@bp.get("/")
def get_feed():
    """
    Unified feed across all sources.

    Query params:
      - limit  (default 50, max 200)
      - offset (default 0)
      - source_type  filter: reddit | twitter | bluesky | manual | file
      - topic        filter: exact topic tag match
      - saved        filter: 'true' to show only saved items
      - sort         'recent' (default) | 'signal'
    """
    limit       = min(int(request.args.get("limit", 50)), 200)
    offset      = int(request.args.get("offset", 0))
    source_type = request.args.get("source_type")
    topic       = request.args.get("topic")
    saved_only  = request.args.get("saved") == "true"
    sort        = request.args.get("sort", "recent")

    filters = ["a.status != 'superseded'", "a.user_status IS DISTINCT FROM 'dismissed'"]
    params: dict = {"limit": limit, "offset": offset}

    if source_type:
        filters.append("s.type = :source_type")
        params["source_type"] = source_type

    if topic:
        # EXISTS subquery avoids any driver-level conflict with the ? JSONB operator
        filters.append(
            "EXISTS (SELECT 1 FROM jsonb_array_elements_text(a.enrichment->'topics') t WHERE t = :topic)"
        )
        params["topic"] = topic

    if saved_only:
        filters.append("a.user_status = 'saved'")

    where = " AND ".join(filters)

    order = "a.created_at DESC" if sort == "recent" else "signal_score DESC, a.created_at DESC"

    query = f"""
        SELECT
            a.id,
            a.type,
            a.label,
            a.content,
            a.source_url,
            a.enrichment,
            a.metadata,
            a.user_status,
            a.created_at,
            s.type   AS source_type,
            s.name   AS source_name,
            (
                -- signal = log(score+1) * 0.4 + freshness * 0.6
                -- freshness: 1.0 = now, 0.0 = 30 days ago
                COALESCE(
                    LN(COALESCE((a.metadata->>'score')::float, 0) + 1) * 0.4
                    + GREATEST(0, 1 - EXTRACT(EPOCH FROM (now() - a.created_at)) / 2592000.0) * 0.6,
                    0
                )
            ) AS signal_score
        FROM artifacts a
        LEFT JOIN sources s ON s.id = a.source_id
        WHERE {where}
        ORDER BY {order}
        LIMIT :limit OFFSET :offset
    """

    count_query = f"""
        SELECT COUNT(*)
        FROM artifacts a
        LEFT JOIN sources s ON s.id = a.source_id
        WHERE {where}
    """

    with get_session() as session:
        rows  = session.execute(text(query), params).fetchall()
        total = session.execute(text(count_query), {k: v for k, v in params.items() if k not in ("limit", "offset")}).scalar()

    return jsonify({
        "items":  [_serialize(r) for r in rows],
        "total":  total,
        "limit":  limit,
        "offset": offset,
    })


@bp.post("/<artifact_id>/save")
def save_artifact(artifact_id: str):
    with get_session() as session:
        session.execute(text(
            "UPDATE artifacts SET user_status = 'saved' WHERE id = :id"
        ), {"id": artifact_id})
        session.commit()
    return jsonify({"ok": True})


@bp.post("/<artifact_id>/dismiss")
def dismiss_artifact(artifact_id: str):
    with get_session() as session:
        session.execute(text(
            "UPDATE artifacts SET user_status = 'dismissed' WHERE id = :id"
        ), {"id": artifact_id})
        session.commit()
    return jsonify({"ok": True})


@bp.post("/<artifact_id>/unsave")
def unsave_artifact(artifact_id: str):
    with get_session() as session:
        session.execute(text(
            "UPDATE artifacts SET user_status = NULL WHERE id = :id"
        ), {"id": artifact_id})
        session.commit()
    return jsonify({"ok": True})


@bp.get("/topics")
def get_topics():
    """Return all distinct enriched topics for filter UI."""
    with get_session() as session:
        rows = session.execute(text("""
            SELECT DISTINCT jsonb_array_elements_text(enrichment->'topics') AS topic
            FROM artifacts
            WHERE enrichment IS NOT NULL
              AND status != 'superseded'
            ORDER BY topic
            LIMIT 100
        """)).fetchall()
    return jsonify([r.topic for r in rows])


@bp.get("/sources")
def get_sources():
    """Return all active sources for filter UI."""
    with get_session() as session:
        rows = session.execute(text("""
            SELECT id, name, type, sync_state, last_synced_at
            FROM sources
            ORDER BY name
        """)).fetchall()
    return jsonify([{
        "id":           str(r.id),
        "name":         r.name,
        "type":         r.type,
        "syncState":    r.sync_state,
        "lastSyncedAt": r.last_synced_at.isoformat() if r.last_synced_at else None,
    } for r in rows])
