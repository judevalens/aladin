"""
Promote enriched artifact data into Neo4j.
Creates artifact nodes, entity nodes, topic nodes, and relationships.
Safe to call multiple times — all writes use MERGE.
"""
from __future__ import annotations
import logging

logger = logging.getLogger(__name__)

# Types that are containers, not meaningful content nodes
_SKIP_TYPES = {"feed"}


def promote(artifact_id: str, artifact_label: str, artifact_type: str,
            source_url: str | None, enrichment: dict) -> None:
    """Push an enriched artifact + its entities/topics into Neo4j."""
    if artifact_type in _SKIP_TYPES:
        return

    try:
        from app.graph.connection import GraphDB
        with GraphDB.session() as session:
            # Upsert artifact node
            session.run(
                """
                MERGE (a:Artifact {id: $id})
                SET a.label = $label,
                    a.type  = $type,
                    a.sourceUrl = $sourceUrl,
                    a.summary = $summary
                """,
                id=artifact_id,
                label=artifact_label,
                type=artifact_type,
                sourceUrl=source_url or "",
                summary=enrichment.get("summary", ""),
            )

            # Entity nodes + MENTIONS edges
            for entity in enrichment.get("entities", []):
                entity = entity.strip()
                if not entity:
                    continue
                session.run(
                    """
                    MERGE (e:Entity {name: $name})
                    WITH e
                    MATCH (a:Artifact {id: $aid})
                    MERGE (a)-[:MENTIONS]->(e)
                    """,
                    name=entity, aid=artifact_id,
                )

            # Topic nodes + TAGGED_WITH edges
            for topic in enrichment.get("topics", []):
                topic = topic.strip()
                if not topic:
                    continue
                session.run(
                    """
                    MERGE (t:Topic {name: $name})
                    WITH t
                    MATCH (a:Artifact {id: $aid})
                    MERGE (a)-[:TAGGED_WITH]->(t)
                    """,
                    name=topic, aid=artifact_id,
                )

        logger.debug("Promoted artifact %s to Neo4j (%d entities, %d topics)",
                     artifact_id,
                     len(enrichment.get("entities", [])),
                     len(enrichment.get("topics", [])))

    except Exception:
        # Neo4j unavailable — don't crash the pipeline
        logger.warning("Neo4j promotion skipped for %s (connection error)", artifact_id, exc_info=True)
