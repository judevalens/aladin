import os
import logging
from contextlib import contextmanager
from neo4j import GraphDatabase, Driver

logger = logging.getLogger(__name__)


class GraphDB:
    _driver: Driver | None = None

    @classmethod
    def is_configured(cls) -> bool:
        return bool(os.environ.get("NEO4J_URI"))

    @classmethod
    def driver(cls) -> Driver:
        if cls._driver is None:
            uri  = os.environ.get("NEO4J_URI")
            user = os.environ.get("NEO4J_USER", "neo4j")
            pw   = os.environ.get("NEO4J_PASSWORD", "")
            if not uri:
                raise RuntimeError("NEO4J_URI is not set — graph features unavailable")
            cls._driver = GraphDatabase.driver(uri, auth=(user, pw))
        return cls._driver

    @classmethod
    def close(cls) -> None:
        if cls._driver is not None:
            cls._driver.close()
            cls._driver = None

    @classmethod
    @contextmanager
    def session(cls):
        with cls.driver().session() as session:
            yield session
