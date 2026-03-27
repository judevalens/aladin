from flask import Blueprint, jsonify, request
from sqlalchemy import text
from app.db import get_session

bp = Blueprint("artifacts", __name__)


def _serialize(row) -> dict:
    return {
        "id":         row.id,
        "type":       row.type,
        "label":      row.label,
        "content":    row.content,
        "sourceUrl":  row.source_url,
        "enrichment": row.enrichment,
        "metadata":   row.metadata_ if hasattr(row, "metadata_") else (row.metadata if hasattr(row, "metadata") else {}),
        "childCount": row.child_count if hasattr(row, "child_count") else 0,
        "createdAt":  row.created_at.isoformat(),
    }


@bp.get("/")
def list_artifacts():
    """Return top-level artifacts only (parent_id IS NULL, not superseded)."""
    with get_session() as session:
        rows = session.execute(text("""
            SELECT
                a.id, a.type, a.label, a.content, a.source_url,
                a.enrichment, a.metadata, a.created_at,
                COUNT(c.id) AS child_count
            FROM artifacts a
            LEFT JOIN artifacts c ON c.parent_id = a.id AND c.status != 'superseded'
            WHERE a.parent_id IS NULL
              AND a.status != 'superseded'
            GROUP BY a.id, a.type, a.label, a.content, a.source_url,
                     a.enrichment, a.metadata, a.created_at
            ORDER BY a.created_at DESC
        """)).fetchall()
        return jsonify([_serialize_row(r) for r in rows])


def _serialize_row(row) -> dict:
    return {
        "id":         row.id,
        "type":       row.type,
        "label":      row.label,
        "content":    row.content,
        "sourceUrl":  row.source_url,
        "enrichment": row.enrichment,
        "metadata":   row.metadata or {},
        "childCount": row.child_count,
        "createdAt":  row.created_at.isoformat(),
    }


@bp.get("/<artifact_id>/children")
def list_children(artifact_id: str):
    """Return child artifacts (posts) for a given parent (feed)."""
    limit  = min(int(request.args.get("limit",  100)), 500)
    offset = int(request.args.get("offset", 0))

    with get_session() as session:
        rows = session.execute(text("""
            SELECT
                id, type, label, content, source_url,
                enrichment, metadata, created_at,
                0 AS child_count
            FROM artifacts
            WHERE parent_id = :parent_id
              AND status != 'superseded'
            ORDER BY
                COALESCE((metadata->>'score')::int, 0) DESC,
                created_at DESC
            LIMIT :limit OFFSET :offset
        """), {"parent_id": artifact_id, "limit": limit, "offset": offset}).fetchall()

        total = session.execute(text("""
            SELECT COUNT(*) FROM artifacts
            WHERE parent_id = :parent_id AND status != 'superseded'
        """), {"parent_id": artifact_id}).scalar()

        return jsonify({
            "items":  [_serialize_row(r) for r in rows],
            "total":  total,
            "limit":  limit,
            "offset": offset,
        })


@bp.post("/")
def create_artifact():
    data = request.get_json()
    from datetime import datetime, timezone
    from app.db.models import Artifact

    with get_session() as session:
        artifact = Artifact(
            id=data["id"],
            type=data["type"],
            label=data["label"],
            content=data.get("content", ""),
            source_url=data.get("sourceUrl"),
            created_at=datetime.now(timezone.utc),
        )
        session.merge(artifact)
        session.commit()
        session.refresh(artifact)
        return jsonify(_serialize_row_model(artifact)), 201


def _serialize_row_model(artifact) -> dict:
    return {
        "id":         artifact.id,
        "type":       artifact.type,
        "label":      artifact.label,
        "content":    artifact.content,
        "sourceUrl":  artifact.source_url,
        "enrichment": artifact.enrichment,
        "metadata":   artifact.metadata_ or {},
        "childCount": 0,
        "createdAt":  artifact.created_at.isoformat(),
    }


@bp.delete("/<artifact_id>")
def delete_artifact(artifact_id: str):
    from app.db.models import Artifact

    with get_session() as session:
        artifact = session.get(Artifact, artifact_id)
        if artifact:
            session.delete(artifact)
            session.commit()
        return jsonify({"ok": True})
