import os
from contextlib import contextmanager
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker, DeclarativeBase

class Base(DeclarativeBase):
    pass

_engine = None
_session_factory = None


def get_engine():
    global _engine
    if _engine is None:
        url = os.environ.get("DATABASE_URL")
        if not url:
            raise RuntimeError("DATABASE_URL is not set")
        _engine = create_engine(url)
    return _engine


def get_session_factory():
    global _session_factory
    if _session_factory is None:
        _session_factory = sessionmaker(bind=get_engine(), autocommit=False, autoflush=False)
    return _session_factory


def get_db():
    """FastAPI / Flask dependency — yields a session."""
    db = get_session_factory()()
    try:
        yield db
    finally:
        db.close()


@contextmanager
def get_session():
    """Context manager for use outside of request context (workers, scheduler)."""
    session = get_session_factory()()
    try:
        yield session
    except Exception:
        session.rollback()
        raise
    finally:
        session.close()
