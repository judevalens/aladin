import random
from flask import jsonify, request
from . import bp
from app.graph import service as graph_service

QUOTES = [
    {"text": "The more that you read, the more things you will know.", "author": "Dr. Seuss"},
    {"text": "Knowledge is power.", "author": "Francis Bacon"},
    {"text": "An investment in knowledge pays the best interest.", "author": "Benjamin Franklin"},
    {"text": "The only true wisdom is in knowing you know nothing.", "author": "Socrates"},
    {"text": "Research is formalized curiosity.", "author": "Zora Neale Hurston"},
    {"text": "The scientist is not a person who gives the right answers, he's one who asks the right questions.", "author": "Claude Lévi-Strauss"},
]


@bp.get("/health")
def health():
    return jsonify({"status": "ok"})


@bp.get("/quote")
def get_quote():
    return jsonify(random.choice(QUOTES))


@bp.get("/graph")
def get_graph():
    return jsonify(graph_service.get_graph())


@bp.post("/graph/nodes")
def create_node():
    data = request.get_json()
    node = graph_service.create_node(
        node_id=data["id"],
        label=data["label"],
        node_type=data.get("type", "default"),
        content=data.get("content", ""),
        x=data.get("x", 0),
        y=data.get("y", 0),
        color=data.get("color", "#3b82f6"),
        source_url=data.get("sourceUrl"),
    )
    return jsonify(node), 201


@bp.patch("/graph/nodes/<node_id>/position")
def update_node_position(node_id: str):
    data = request.get_json()
    updated = graph_service.update_node_position(node_id, data["x"], data["y"])
    if not updated:
        return jsonify({"error": "Node not found"}), 404
    return jsonify({"ok": True})


@bp.post("/graph/edges")
def create_edge():
    data = request.get_json()
    edge = graph_service.create_edge(
        edge_id=data["id"],
        source_id=data["source"],
        target_id=data["target"],
    )
    if not edge:
        return jsonify({"error": "One or both nodes not found"}), 404
    return jsonify(edge), 201


@bp.delete("/graph/nodes/<node_id>")
def delete_node(node_id: str):
    graph_service.delete_node(node_id)
    return jsonify({"ok": True})


@bp.delete("/graph/edges/<edge_id>")
def delete_edge(edge_id: str):
    graph_service.delete_edge(edge_id)
    return jsonify({"ok": True})
