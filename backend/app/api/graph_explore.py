"""
Graph exploration API — neighborhood traversal, entity pages, topic clusters.
Uses Neo4j for graph queries on pipeline-promoted nodes.

GET /api/graph-explore/neighborhood/<node_id>?depth=1
GET /api/graph-explore/entities?limit=50
GET /api/graph-explore/topics?limit=30
GET /api/graph-explore/entity/<name>         - entity page: artifacts + neighbors
GET /api/graph-explore/full                  - merged graph (manual + pipeline nodes)
"""
import uuid
from flask import Blueprint, jsonify, request

bp = Blueprint("graph_explore", __name__)


def _neo4j_unavailable():
    from app.graph.connection import GraphDB
    return not GraphDB.is_configured()


@bp.get("/neighborhood/<node_id>")
def neighborhood(node_id: str):
    """
    Return the subgraph around a node (artifact or entity) up to `depth` hops.
    """
    depth = min(int(request.args.get("depth", 1)), 3)

    if _neo4j_unavailable():
        return jsonify({"nodes": [], "edges": [], "center": node_id})

    try:
        from app.graph.connection import GraphDB
        with GraphDB.session() as session:
            # Find the center node
            center_result = session.run("""
                MATCH (n)
                WHERE n.id = $node_id OR n.name = $node_id
                RETURN n LIMIT 1
            """, node_id=node_id).single()

            if not center_result:
                return jsonify({"nodes": [], "edges": [], "center": node_id})

            center_node = center_result["n"]

            # Get all neighbor nodes within depth hops
            neighbor_result = session.run("""
                MATCH (center)
                WHERE center.id = $node_id OR center.name = $node_id
                MATCH (center)-[*1..$depth]-(neighbor)
                RETURN COLLECT(DISTINCT neighbor) AS neighbors
            """, node_id=node_id, depth=depth).single()

            # Get all direct relationships from center (for edges)
            rel_result = session.run("""
                MATCH (center)
                WHERE center.id = $node_id OR center.name = $node_id
                MATCH (center)-[r]-(neighbor)
                RETURN COLLECT(DISTINCT r) AS rels
            """, node_id=node_id).single()

            all_nodes = [center_node] + list(neighbor_result["neighbors"] if neighbor_result else [])
            all_rels  = list(rel_result["rels"] if rel_result else [])

        nodes = [_serialize_node(n) for n in all_nodes]
        edges = [_serialize_rel(r)  for r in all_rels]
        return jsonify({"nodes": nodes, "edges": edges, "center": node_id})
    except Exception as e:
        return jsonify({"error": str(e), "nodes": [], "edges": []}), 500


@bp.get("/full")
def full_graph():
    """
    Merged graph: manual GraphNodes + pipeline-promoted Artifacts/Entities/Topics.
    Used by the D3 canvas to show the full knowledge graph.
    """
    if _neo4j_unavailable():
        # Fall back to Postgres-only artifact data
        return _postgres_graph()

    try:
        from app.graph.connection import GraphDB
        with GraphDB.session() as session:
            # Manual nodes (user-placed in D3 canvas)
            manual_nodes = session.run(
                "MATCH (n:GraphNode) RETURN n.id AS id, n.label AS label, n.type AS type, "
                "n.content AS content, n.sourceUrl AS sourceUrl, n.x AS x, n.y AS y, n.color AS color"
            ).data()

            manual_edges = session.run(
                "MATCH (a:GraphNode)-[r:CONNECTED]->(b:GraphNode) "
                "RETURN r.id AS id, a.id AS source, b.id AS target"
            ).data()

            # Pipeline-promoted nodes (limited to 200 for performance)
            pipeline_nodes = session.run("""
                MATCH (n)
                WHERE n:Artifact OR n:Entity OR n:Topic
                RETURN
                    COALESCE(n.id, n.name) AS id,
                    COALESCE(n.label, n.name) AS label,
                    CASE
                        WHEN n:Artifact THEN 'artifact'
                        WHEN n:Entity   THEN 'entity'
                        WHEN n:Topic    THEN 'topic'
                    END AS type,
                    COALESCE(n.summary, '') AS content,
                    COALESCE(n.sourceUrl, '') AS sourceUrl
                LIMIT 200
            """).data()

            pipeline_edges = session.run("""
                MATCH (a)-[r]->(b)
                WHERE type(r) IN ['MENTIONS', 'TAGGED_WITH']
                  AND (a:Artifact OR a:Entity OR a:Topic)
                  AND (b:Artifact OR b:Entity OR b:Topic)
                RETURN
                    elementId(r)            AS id,
                    COALESCE(a.id, a.name)  AS source,
                    COALESCE(b.id, b.name)  AS target,
                    type(r)                 AS rel_type
                LIMIT 500
            """).data()

        # Assign visual properties to pipeline nodes
        color_map = {"artifact": "#3b82f6", "entity": "#10b981", "topic": "#f59e0b"}
        for n in pipeline_nodes:
            n["color"]     = color_map.get(n.get("type", ""), "#94a3b8")
            n["x"]         = 0
            n["y"]         = 0
            n["pipeline"]  = True   # flag so frontend can style differently

        for e in pipeline_edges:
            e["id"]      = str(e.get("id", uuid.uuid4()))
            e["relType"] = e.pop("rel_type", "CONNECTED")

        all_nodes = manual_nodes + pipeline_nodes
        all_edges = manual_edges + pipeline_edges

        return jsonify({"nodes": all_nodes, "edges": all_edges})

    except Exception as e:
        return jsonify({"error": str(e), "nodes": [], "edges": []})


@bp.get("/entities")
def list_entities():
    """All entity nodes in the knowledge graph with artifact counts."""
    if _neo4j_unavailable():
        return jsonify([])

    try:
        from app.graph.connection import GraphDB
        with GraphDB.session() as session:
            limit = min(int(request.args.get("limit", 50)), 200)
            rows = session.run("""
                MATCH (e:Entity)<-[:MENTIONS]-(a:Artifact)
                RETURN e.name AS name, COUNT(DISTINCT a) AS artifact_count
                ORDER BY artifact_count DESC
                LIMIT $limit
            """, limit=limit).data()
        return jsonify(rows)
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@bp.get("/topics")
def list_topics():
    """All topic nodes with artifact counts."""
    if _neo4j_unavailable():
        # Fall back to Postgres enrichment data
        from app.db import get_session
        from sqlalchemy import text
        with get_session() as session:
            rows = session.execute(text("""
                SELECT
                    jsonb_array_elements_text(enrichment->'topics') AS name,
                    COUNT(*) AS artifact_count
                FROM artifacts
                WHERE enrichment IS NOT NULL AND status != 'superseded'
                GROUP BY name
                ORDER BY artifact_count DESC
                LIMIT 50
            """)).fetchall()
        return jsonify([{"name": r.name, "artifact_count": r.artifact_count} for r in rows])

    try:
        from app.graph.connection import GraphDB
        limit = min(int(request.args.get("limit", 30)), 100)
        with GraphDB.session() as session:
            rows = session.run("""
                MATCH (t:Topic)<-[:TAGGED_WITH]-(a:Artifact)
                RETURN t.name AS name, COUNT(DISTINCT a) AS artifact_count
                ORDER BY artifact_count DESC
                LIMIT $limit
            """, limit=limit).data()
        return jsonify(rows)
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@bp.get("/entity/<path:entity_name>")
def entity_page(entity_name: str):
    """
    Entity page: all artifacts mentioning this entity,
    and neighboring entities (co-mentioned).
    Falls back to Postgres enrichment if Neo4j unavailable.
    """
    if _neo4j_unavailable():
        return _entity_from_postgres(entity_name)

    try:
        from app.graph.connection import GraphDB
        with GraphDB.session() as session:
            artifacts = session.run("""
                MATCH (e:Entity {name: $name})<-[:MENTIONS]-(a:Artifact)
                RETURN a.id AS id, a.label AS label, a.type AS type, a.summary AS summary
                LIMIT 50
            """, name=entity_name).data()

            neighbors = session.run("""
                MATCH (e:Entity {name: $name})<-[:MENTIONS]-(a:Artifact)-[:MENTIONS]->(other:Entity)
                WHERE other.name <> $name
                RETURN other.name AS name, COUNT(DISTINCT a) AS shared_artifacts
                ORDER BY shared_artifacts DESC
                LIMIT 20
            """, name=entity_name).data()

        return jsonify({
            "entity":    entity_name,
            "artifacts": artifacts,
            "neighbors": neighbors,
        })
    except Exception as e:
        return jsonify({"error": str(e)}), 500


# ── Fallbacks using Postgres ──────────────────────────────────────────────────

def _postgres_graph():
    """Minimal graph from Postgres enrichment data when Neo4j is unavailable."""
    from app.db import get_session
    from sqlalchemy import text

    with get_session() as session:
        rows = session.execute(text("""
            SELECT id, type, label, source_url, enrichment
            FROM artifacts
            WHERE enrichment IS NOT NULL
              AND status NOT IN ('superseded', 'dismissed')
            ORDER BY created_at DESC
            LIMIT 100
        """)).fetchall()

    nodes, edges = [], []
    entity_seen: dict = {}

    for row in rows:
        nodes.append({
            "id":        row.id,
            "label":     row.label,
            "type":      row.type,
            "content":   (row.enrichment or {}).get("summary", ""),
            "sourceUrl": row.source_url or "",
            "color":     "#3b82f6",
            "x": 0, "y": 0,
            "pipeline": True,
        })
        enrichment = row.enrichment or {}
        for entity in (enrichment.get("entities") or [])[:5]:
            eid = f"entity:{entity}"
            if eid not in entity_seen:
                entity_seen[eid] = True
                nodes.append({
                    "id": eid, "label": entity, "type": "entity",
                    "content": "", "sourceUrl": "", "color": "#10b981",
                    "x": 0, "y": 0, "pipeline": True,
                })
            edges.append({
                "id":      str(uuid.uuid4()),
                "source":  row.id,
                "target":  eid,
                "relType": "MENTIONS",
            })

    return jsonify({"nodes": nodes, "edges": edges})


def _entity_from_postgres(entity_name: str):
    from app.db import get_session
    from sqlalchemy import text
    with get_session() as session:
        rows = session.execute(text("""
            SELECT id, type, label, enrichment
            FROM artifacts
            WHERE EXISTS (
                SELECT 1 FROM jsonb_array_elements_text(enrichment->'entities') e WHERE e = :name
            )
              AND status != 'superseded'
            ORDER BY created_at DESC
            LIMIT 30
        """), {"name": entity_name}).fetchall()
    return jsonify({
        "entity":    entity_name,
        "artifacts": [{"id": r.id, "label": r.label, "type": r.type} for r in rows],
        "neighbors": [],
    })


# ── Serializers ───────────────────────────────────────────────────────────────

def _serialize_node(n) -> dict:
    labels = list(n.labels)
    node_type = "entity" if "Entity" in labels else "topic" if "Topic" in labels else "artifact"
    color_map = {"artifact": "#3b82f6", "entity": "#10b981", "topic": "#f59e0b"}
    return {
        "id":       n.get("id") or n.get("name", ""),
        "label":    n.get("label") or n.get("name", ""),
        "type":     node_type,
        "content":  n.get("summary", "") or "",
        "color":    color_map.get(node_type, "#94a3b8"),
        "x": 0, "y": 0,
    }


def _serialize_rel(r) -> dict:
    return {
        "id":      str(r.element_id),
        "source":  r.start_node.get("id") or r.start_node.get("name", ""),
        "target":  r.end_node.get("id")   or r.end_node.get("name", ""),
        "relType": r.type,
    }
