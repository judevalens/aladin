"""
Insights API — read and act on generated insights.

GET  /api/insights
POST /api/insights/<id>/accept
POST /api/insights/<id>/dismiss
GET  /api/insights/stats
"""
from flask import Blueprint, jsonify, request
from sqlalchemy import text
from app.db import get_session

bp = Blueprint("insights", __name__)


def _serialize(row) -> dict:
    return {
        "id":           str(row.id),
        "type":         row.type,
        "title":        row.title,
        "body":         row.body,
        "entity":       row.entity,
        "topic":        row.topic,
        "artifactIds":  row.artifact_ids or [],
        "confidence":   float(row.confidence),
        "userStatus":   row.user_status,
        "createdAt":    row.created_at.isoformat(),
    }


@bp.get("/")
def list_insights():
    """
    Return insights, newest first.

    Query params:
      - limit       (default 30, max 100)
      - offset      (default 0)
      - type        bridge | convergence | trend | contradiction
      - status      pending | accepted | dismissed (default: pending)
    """
    limit       = min(int(request.args.get("limit", 30)), 100)
    offset      = int(request.args.get("offset", 0))
    insight_type = request.args.get("type")
    status       = request.args.get("status", "pending")

    filters = []
    params: dict = {"limit": limit, "offset": offset}

    if insight_type:
        filters.append("type = :insight_type")
        params["insight_type"] = insight_type

    if status:
        filters.append("user_status = :status")
        params["status"] = status

    where = ("WHERE " + " AND ".join(filters)) if filters else ""

    with get_session() as session:
        rows = session.execute(text(f"""
            SELECT id, type, title, body, entity, topic,
                   artifact_ids, confidence, user_status, created_at
            FROM insights
            {where}
            ORDER BY confidence DESC, created_at DESC
            LIMIT :limit OFFSET :offset
        """), params).fetchall()

        total = session.execute(text(f"""
            SELECT COUNT(*) FROM insights {where}
        """), {k: v for k, v in params.items() if k not in ("limit", "offset")}).scalar()

    return jsonify({
        "items":  [_serialize(r) for r in rows],
        "total":  total,
        "limit":  limit,
        "offset": offset,
    })


@bp.post("/<insight_id>/accept")
def accept_insight(insight_id: str):
    with get_session() as session:
        session.execute(text(
            "UPDATE insights SET user_status = 'accepted' WHERE id = CAST(:id AS uuid)"
        ), {"id": insight_id})
        session.commit()
    return jsonify({"ok": True})


@bp.post("/<insight_id>/dismiss")
def dismiss_insight(insight_id: str):
    with get_session() as session:
        session.execute(text(
            "UPDATE insights SET user_status = 'dismissed' WHERE id = CAST(:id AS uuid)"
        ), {"id": insight_id})
        session.commit()
    return jsonify({"ok": True})


@bp.get("/stats")
def insight_stats():
    """Summary counts by type and status."""
    with get_session() as session:
        rows = session.execute(text("""
            SELECT type, user_status, COUNT(*) AS count
            FROM insights
            GROUP BY type, user_status
        """)).fetchall()

    by_type: dict   = {}
    by_status: dict = {}
    for r in rows:
        by_type[r.type]       = by_type.get(r.type, 0) + r.count
        by_status[r.user_status] = by_status.get(r.user_status, 0) + r.count

    return jsonify({"byType": by_type, "byStatus": by_status})
