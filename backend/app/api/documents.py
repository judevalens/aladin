from http import HTTPStatus
from pathlib import Path
from flask import Blueprint, request, jsonify, send_file
from app.db import get_session_factory
from app.db.models import Document
from app.ingest.pipeline import ingest, UPLOAD_DIR

bp = Blueprint("documents", __name__)


@bp.post("/upload")
def upload():
    file = request.files.get("file")
    if not file:
        return jsonify({"error": "no file provided"}), 400
    if not file.filename.lower().endswith(".pdf"):
        return jsonify({"error": "only PDF files are supported"}), 400

    result = ingest(file.read(), file.filename)
    return jsonify(result), HTTPStatus.CREATED


@bp.get("/")
def list_documents():
    db = get_session_factory()()
    try:
        docs = (
            db.query(Document)
            .order_by(Document.uploaded_at.desc())
            .all()
        )
        return jsonify([{
            "id": doc.id,
            "filename": doc.filename,
            "title": doc.title,
            "size": doc.size,
            "page_count": doc.page_count,
            "uploaded_at": doc.uploaded_at.isoformat(),
            "ingested": doc.ingested,
        } for doc in docs])
    finally:
        db.close()


@bp.get("/<path:doc_id>/file")
def get_file(doc_id: str):
    file_path = UPLOAD_DIR / f"{doc_id}.pdf"
    if not file_path.exists():
        return jsonify({"error": "not found"}), 404
    return send_file(file_path, mimetype="application/pdf")
