import uuid
from pathlib import Path
from flask import Blueprint, request, jsonify, send_file

bp = Blueprint("audio", __name__)

AUDIO_DIR = Path(__file__).resolve().parent.parent.parent / "audio"
AUDIO_DIR.mkdir(exist_ok=True)


@bp.post("/upload")
def upload():
    file = request.files.get("file")
    if not file:
        return jsonify({"error": "no file provided"}), 400

    filename = f"{uuid.uuid4()}.webm"
    file.save(AUDIO_DIR / filename)
    return jsonify({"filename": filename}), 201


@bp.get("/<filename>")
def serve(filename: str):
    path = AUDIO_DIR / filename
    if not path.exists():
        return jsonify({"error": "not found"}), 404
    return send_file(path, mimetype="audio/webm")
