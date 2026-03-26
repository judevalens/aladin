from flask import Flask
from flask_cors import CORS
from dotenv import load_dotenv
import os

load_dotenv()


def create_app() -> Flask:
    app = Flask(__name__)
    app.config["SECRET_KEY"] = os.getenv("SECRET_KEY", "dev-secret")

    origins = os.getenv("CORS_ORIGINS", "*")
    CORS(app, origins=origins)

    from .api import bp as api_bp
    app.register_blueprint(api_bp, url_prefix="/api")

    from .api.documents import bp as docs_bp
    app.register_blueprint(docs_bp, url_prefix="/api/documents")

    from .api.artifacts import bp as artifacts_bp
    app.register_blueprint(artifacts_bp, url_prefix="/api/artifacts")

    from .api.audio import bp as audio_bp
    app.register_blueprint(audio_bp, url_prefix="/api/audio")

    from .api.ingest import bp as ingest_bp
    app.register_blueprint(ingest_bp, url_prefix="/api/ingest")

    from .api.pipeline import bp as pipeline_bp
    app.register_blueprint(pipeline_bp, url_prefix="/api/pipeline")

    _seed()

    return app


def _seed() -> None:
    if not os.getenv("DATABASE_URL"):
        return
    try:
        from .db.seed import seed_default_user
        seed_default_user()
    except Exception as e:
        print(f"[seed] skipped: {e}")
