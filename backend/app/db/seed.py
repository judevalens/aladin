import uuid
from app.db import get_session_factory
from app.db.models import User

DEFAULT_USER_ID = uuid.UUID("00000000-0000-0000-0000-000000000001")
DEFAULT_USER_EMAIL = "dev@aladin.local"


def seed_default_user() -> None:
    db = get_session_factory()()
    try:
        user = db.get(User, DEFAULT_USER_ID)
        if not user:
            db.add(User(id=DEFAULT_USER_ID, email=DEFAULT_USER_EMAIL))
            db.commit()
            print(f"[seed] created default user: {DEFAULT_USER_EMAIL}")
    finally:
        db.close()
