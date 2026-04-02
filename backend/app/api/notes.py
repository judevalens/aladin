"""
Notes API — create, update, delete notes + find related artifacts.

Notes are artifacts with type='note', parent_id=NULL.
They enter the same enrichment pipeline as any other artifact.

POST   /api/notes
PATCH  /api/notes/<id>
DELETE /api/notes/<id>
GET    /api/notes/<id>/related
"""
import uuid
from datetime import datetime, timezone
from flask import Blueprint, jsonify, request
from sqlalchemy import text
from app.db import get_session
from app.db.models import Artifact

bp = Blueprint("notes", __name__)


def _serialize(artifact: Artifact) -> dict:
    return {
        "id":         artifact.id,
        "type":       artifact.type,
        "label":      artifact.label,
        "content":    artifact.content,
        "enrichment": artifact.enrichment,
        "metadata":   artifact.metadata_ or {},
        "status":     artifact.status,
        "createdAt":  artifact.created_at.isoformat(),
    }


@bp.post("/")
def create_note():
    data    = request.get_json() or {}
    label   = (data.get("label") or "").strip()
    content = (data.get("content") or "").strip()

    if not content:
        return jsonify({"error": "content is required"}), 400

    if not label:
        label = content[:60].replace("\n", " ") + ("…" if len(content) > 60 else "")

    artifact_id = f"note-{uuid.uuid4().hex}"

    with get_session() as session:
        artifact = Artifact(
            id=artifact_id,
            type="note",
            label=label,
            content=content,
            status="pending",
            created_at=datetime.now(timezone.utc),
        )
        session.add(artifact)
        session.commit()
        session.refresh(artifact)
        return jsonify(_serialize(artifact)), 201


@bp.patch("/<note_id>")
def update_note(note_id: str):
    data = request.get_json() or {}

    with get_session() as session:
        artifact = session.get(Artifact, note_id)
        if not artifact or artifact.type != "note":
            return jsonify({"error": "Note not found"}), 404

        if "label" in data:
            artifact.label = data["label"].strip() or artifact.label
        if "content" in data:
            content = data["content"].strip()
            if content != artifact.content:
                artifact.content = content
                artifact.status  = "pending"   # re-enrich on edit
                artifact.enrichment = None
                artifact.embedding  = None

        session.commit()
        session.refresh(artifact)
        return jsonify(_serialize(artifact))


@bp.delete("/<note_id>")
def delete_note(note_id: str):
    with get_session() as session:
        artifact = session.get(Artifact, note_id)
        if artifact:
            session.delete(artifact)
            session.commit()
    return jsonify({"ok": True})


@bp.get("/<note_id>/related")
def get_related(note_id: str):
    """Return artifacts semantically related to this note."""
    limit = min(int(request.args.get("limit", 8)), 30)

    with get_session() as session:
        row = session.execute(text(
            "SELECT embedding FROM artifacts WHERE id = :id"
        ), {"id": note_id}).fetchone()

    if not row or row.embedding is None:
        return jsonify({"results": [], "message": "Note not yet embedded"})

    query = """
        SELECT
            a.id, a.type, a.label, a.content, a.source_url,
            a.enrichment, a.metadata, a.created_at,
            1 - (a.embedding <=> CAST(:vec AS vector)) AS similarity
        FROM artifacts a
        WHERE a.embedding IS NOT NULL
          AND a.status != 'superseded'
          AND a.id != :note_id
        ORDER BY a.embedding <=> CAST(:vec AS vector)
        LIMIT :limit
    """

    with get_session() as session:
        rows = session.execute(text(query), {
            "vec":     str(row.embedding),
            "note_id": note_id,
            "limit":   limit,
        }).fetchall()

    return jsonify({"results": [{
        "id":         r.id,
        "type":       r.type,
        "label":      r.label,
        "content":    r.content[:300] if r.content else "",
        "sourceUrl":  r.source_url,
        "enrichment": r.enrichment,
        "similarity": float(r.similarity),
        "createdAt":  r.created_at.isoformat(),
    } for r in rows]})
