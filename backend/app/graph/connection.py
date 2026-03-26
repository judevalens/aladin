import os
from contextlib import contextmanager
from neo4j import GraphDatabase, Driver


class GraphDB:
    _driver: Driver | None = None

    @classmethod
    def driver(cls) -> Driver:
        if cls._driver is None:
            cls._driver = GraphDatabase.driver(
                os.environ["NEO4J_URI"],
                auth=(os.environ["NEO4J_USER"], os.environ["NEO4J_PASSWORD"]),
            )
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
