import json
from flask import Blueprint, jsonify, request, Response, stream_with_context
from app.db import get_session_factory
from app.db.models import Artifact
from app.pipeline.enrichment import enrich
from app.pipeline.chat import stream_chat

bp = Blueprint("pipeline", __name__)


@bp.post("/enrich/<artifact_id>")
def enrich_artifact(artifact_id: str):
    db = get_session_factory()()
    try:
        artifact = db.get(Artifact, artifact_id)
        if not artifact:
            return jsonify({"error": "Artifact not found"}), 404

        result = enrich(content=artifact.content, artifact_type=artifact.type)
        artifact.enrichment = result.model_dump()
        db.commit()

        return jsonify({"enrichment": artifact.enrichment})
    except Exception as e:
        return jsonify({"error": str(e)}), 500
    finally:
        db.close()


@bp.post("/chat")
def chat():
    data = request.get_json()
    message = (data or {}).get("message", "").strip()
    history = (data or {}).get("history", [])

    if not message:
        return jsonify({"error": "message is required"}), 400

    db = get_session_factory()()
    try:
        artifacts = db.query(Artifact).order_by(Artifact.created_at.desc()).all()
        artifact_dicts = [{
            "id": a.id,
            "type": a.type,
            "label": a.label,
            "content": a.content,
            "enrichment": a.enrichment,
        } for a in artifacts]
    finally:
        db.close()

    def generate():
        try:
            for token in stream_chat(message, history, artifact_dicts):
                yield f"data: {json.dumps({'token': token})}\n\n"
        except Exception as e:
            yield f"data: {json.dumps({'error': str(e)})}\n\n"
        yield "data: [DONE]\n\n"

    return Response(
        stream_with_context(generate()),
        mimetype="text/event-stream",
        headers={"X-Accel-Buffering": "no"},
    )
