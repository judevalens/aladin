import uuid
from app.db import get_session_factory
from app.db.models import User, KnowledgeGraph

DEFAULT_USER_ID = uuid.UUID("00000000-0000-0000-0000-000000000001")
DEFAULT_USER_EMAIL = "dev@aladin.local"
DEFAULT_KG_NAME = "Default Research Graph"


def seed_default_user() -> None:
    db = get_session_factory()()
    try:
        user = db.get(User, DEFAULT_USER_ID)
        if not user:
            db.add(User(id=DEFAULT_USER_ID, email=DEFAULT_USER_EMAIL))
            print(f"[seed] created default user: {DEFAULT_USER_EMAIL}")

        kg = db.query(KnowledgeGraph).filter(KnowledgeGraph.user_id == DEFAULT_USER_ID).first()
        if not kg:
            db.add(KnowledgeGraph(
                user_id=DEFAULT_USER_ID,
                name=DEFAULT_KG_NAME,
                description="Default workspace for persisted feed sources.",
            ))
            print(f"[seed] created default knowledge graph: {DEFAULT_KG_NAME}")

        db.commit()
    finally:
        db.close()
