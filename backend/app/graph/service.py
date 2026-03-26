from .connection import GraphDB


def get_graph() -> dict:
    with GraphDB.session() as session:
        nodes_result = session.run(
            "MATCH (n:GraphNode) RETURN n.id AS id, n.label AS label, n.type AS type, "
            "n.content AS content, n.sourceUrl AS sourceUrl, n.x AS x, n.y AS y, n.color AS color"
        )
        edges_result = session.run(
            "MATCH (a:GraphNode)-[r:CONNECTED]->(b:GraphNode) "
            "RETURN r.id AS id, a.id AS source, b.id AS target"
        )
        nodes = [dict(record) for record in nodes_result]
        edges = [dict(record) for record in edges_result]
    return {"nodes": nodes, "edges": edges}


def create_node(node_id: str, label: str, node_type: str, content: str, x: float, y: float, color: str, source_url: str | None = None) -> dict:
    with GraphDB.session() as session:
        result = session.run(
            "MERGE (n:GraphNode {id: $id}) "
            "SET n.label = $label, n.type = $type, n.content = $content, "
            "n.sourceUrl = $sourceUrl, n.x = $x, n.y = $y, n.color = $color "
            "RETURN n.id AS id, n.label AS label, n.type AS type, "
            "n.content AS content, n.sourceUrl AS sourceUrl, n.x AS x, n.y AS y, n.color AS color",
            id=node_id, label=label, type=node_type, content=content,
            sourceUrl=source_url, x=x, y=y, color=color,
        )
        return dict(result.single())


def update_node_position(node_id: str, x: float, y: float) -> bool:
    with GraphDB.session() as session:
        result = session.run(
            "MATCH (n:GraphNode {id: $id}) SET n.x = $x, n.y = $y RETURN n.id",
            id=node_id, x=x, y=y,
        )
        return result.single() is not None


def create_edge(edge_id: str, source_id: str, target_id: str) -> dict:
    with GraphDB.session() as session:
        result = session.run(
            "MATCH (a:GraphNode {id: $source}), (b:GraphNode {id: $target}) "
            "MERGE (a)-[r:CONNECTED {id: $id}]->(b) "
            "RETURN r.id AS id, a.id AS source, b.id AS target",
            id=edge_id, source=source_id, target=target_id,
        )
        record = result.single()
        if record is None:
            return {}
        return dict(record)


def delete_node(node_id: str) -> bool:
    with GraphDB.session() as session:
        result = session.run(
            "MATCH (n:GraphNode {id: $id}) DETACH DELETE n RETURN count(n) AS deleted",
            id=node_id,
        )
        return result.single()["deleted"] > 0


def delete_edge(edge_id: str) -> bool:
    with GraphDB.session() as session:
        result = session.run(
            "MATCH ()-[r:CONNECTED {id: $id}]->() DELETE r RETURN count(r) AS deleted",
            id=edge_id,
        )
        return result.single()["deleted"] > 0
