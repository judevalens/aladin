from flask import Blueprint, jsonify, request
from app.db import get_session_factory
from app.db.models import Artifact
from datetime import datetime, timezone

bp = Blueprint("artifacts", __name__)


def _serialize(artifact: Artifact) -> dict:
    return {
        "id": artifact.id,
        "type": artifact.type,
        "label": artifact.label,
        "content": artifact.content,
        "sourceUrl": artifact.source_url,
        "enrichment": artifact.enrichment,
        "createdAt": artifact.created_at.isoformat(),
    }


@bp.get("/")
def list_artifacts():
    db = get_session_factory()()
    try:
        artifacts = db.query(Artifact).order_by(Artifact.created_at.desc()).all()
        return jsonify([_serialize(a) for a in artifacts])
    finally:
        db.close()


@bp.post("/")
def create_artifact():
    data = request.get_json()
    db = get_session_factory()()
    try:
        artifact = Artifact(
            id=data["id"],
            type=data["type"],
            label=data["label"],
            content=data.get("content", ""),
            source_url=data.get("sourceUrl"),
            created_at=datetime.now(timezone.utc),
        )
        db.merge(artifact)
        db.commit()
        db.refresh(artifact)
        return jsonify(_serialize(artifact)), 201
    finally:
        db.close()


@bp.delete("/<artifact_id>")
def delete_artifact(artifact_id: str):
    db = get_session_factory()()
    try:
        artifact = db.get(Artifact, artifact_id)
        if artifact:
            db.delete(artifact)
            db.commit()
        return jsonify({"ok": True})
    finally:
        db.close()
