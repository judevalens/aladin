"""
Semantic search API — vector similarity over embedded artifacts.

GET  /api/search?q=<query>&limit=10&source_type=reddit
POST /api/search/similar  { artifact_id, limit }
"""
from flask import Blueprint, jsonify, request
from sqlalchemy import text
from app.db import get_session

bp = Blueprint("search", __name__)


def _serialize(row) -> dict:
    return {
        "id":         row.id,
        "type":       row.type,
        "label":      row.label,
        "content":    row.content[:500] if row.content else "",
        "sourceUrl":  row.source_url,
        "enrichment": row.enrichment,
        "metadata":   row.metadata or {},
        "similarity": float(row.similarity) if row.similarity is not None else 0.0,
        "createdAt":  row.created_at.isoformat(),
    }


@bp.get("/")
def semantic_search():
    """
    Semantic search over embedded artifacts.

    Query params:
      - q           (required) search query
      - limit       (default 15, max 50)
      - source_type optional source type filter
      - threshold   minimum similarity score (default 0.3)
    """
    q           = request.args.get("q", "").strip()
    limit       = min(int(request.args.get("limit", 15)), 50)
    source_type = request.args.get("source_type")
    threshold   = float(request.args.get("threshold", 0.3))

    if not q:
        return jsonify({"error": "q is required"}), 400

    try:
        from app.pipeline.embedding import embed_text
        query_vec = embed_text(q)
    except Exception as e:
        return jsonify({"error": f"Embedding failed: {e}"}), 500

    filters = [
        "a.embedding IS NOT NULL",
        "a.status != 'superseded'",
        "1 - (a.embedding <=> CAST(:vec AS vector)) >= :threshold",
    ]
    params: dict = {
        "vec":       str(query_vec),
        "threshold": threshold,
        "limit":     limit,
    }

    if source_type:
        filters.append("s.type = :source_type")
        params["source_type"] = source_type

    where = " AND ".join(filters)

    query = f"""
        SELECT
            a.id, a.type, a.label, a.content, a.source_url,
            a.enrichment, a.metadata, a.created_at,
            1 - (a.embedding <=> CAST(:vec AS vector)) AS similarity
        FROM artifacts a
        LEFT JOIN sources s ON s.id = a.source_id
        WHERE {where}
        ORDER BY a.embedding <=> CAST(:vec AS vector)
        LIMIT :limit
    """

    with get_session() as session:
        rows = session.execute(text(query), params).fetchall()

    return jsonify({
        "query":   q,
        "results": [_serialize(r) for r in rows],
    })


@bp.post("/similar")
def find_similar():
    """Find artifacts similar to a given artifact_id."""
    data        = request.get_json() or {}
    artifact_id = data.get("artifact_id")
    limit       = min(int(data.get("limit", 10)), 50)

    if not artifact_id:
        return jsonify({"error": "artifact_id is required"}), 400

    with get_session() as session:
        row = session.execute(text(
            "SELECT embedding FROM artifacts WHERE id = :id"
        ), {"id": artifact_id}).fetchone()

    if not row or row.embedding is None:
        return jsonify({"error": "Artifact not found or not yet embedded"}), 404

    query = """
        SELECT
            a.id, a.type, a.label, a.content, a.source_url,
            a.enrichment, a.metadata, a.created_at,
            1 - (a.embedding <=> CAST(:vec AS vector)) AS similarity
        FROM artifacts a
        WHERE a.embedding IS NOT NULL
          AND a.status != 'superseded'
          AND a.id != :artifact_id
        ORDER BY a.embedding <=> CAST(:vec AS vector)
        LIMIT :limit
    """

    with get_session() as session:
        rows = session.execute(text(query), {
            "vec":         str(row.embedding),
            "artifact_id": artifact_id,
            "limit":       limit,
        }).fetchall()

    return jsonify({"results": [_serialize(r) for r in rows]})
