"""
Streaming chat backed by semantic RAG.

Embeds the user's message, finds top-K relevant artifacts via pgvector,
and passes them as context to the LLM. Falls back to recency-ranked
artifacts if embeddings are not available.
"""
from __future__ import annotations
import logging
from typing import Generator

from langchain_openai import ChatOpenAI
from langchain_core.messages import SystemMessage, HumanMessage, AIMessage

logger = logging.getLogger(__name__)

_TOP_K = 12   # number of artifacts to retrieve for context


def _semantic_context(message: str) -> list[dict]:
    """Return top-K artifacts semantically relevant to message."""
    try:
        from app.pipeline.embedding import embed_text
        from app.db import get_session
        from sqlalchemy import text

        query_vec = embed_text(message)

        with get_session() as session:
            rows = session.execute(text("""
                SELECT id, type, label, content, enrichment,
                       1 - (embedding <=> CAST(:vec AS vector)) AS similarity
                FROM artifacts
                WHERE embedding IS NOT NULL
                  AND status != 'superseded'
                ORDER BY embedding <=> CAST(:vec AS vector)
                LIMIT :k
            """), {"vec": str(query_vec), "k": _TOP_K}).fetchall()

        return [{
            "id":         r.id,
            "type":       r.type,
            "label":      r.label,
            "content":    r.content,
            "enrichment": r.enrichment or {},
            "similarity": float(r.similarity),
        } for r in rows]

    except Exception as e:
        logger.warning("Semantic context retrieval failed: %s — falling back to recency", e)
        return []


def _recency_context(artifacts: list[dict]) -> list[dict]:
    """Fall back: use caller-supplied artifacts (top K by position)."""
    return artifacts[:_TOP_K]


def _build_context(artifacts: list[dict]) -> str:
    if not artifacts:
        return "No relevant artifacts found in the collection."

    lines = []
    for a in artifacts:
        enrichment = a.get("enrichment") or {}
        summary    = enrichment.get("summary") or a.get("content", "")[:200]
        topics     = ", ".join(enrichment.get("topics") or [])
        line = f"- [{a['type'].upper()}] {a['label']}"
        if topics:
            line += f" ({topics})"
        line += f": {summary}"
        lines.append(line)

    return "\n".join(lines)


def stream_chat(
    message: str,
    history: list[dict],
    artifacts: list[dict],
) -> Generator[str, None, None]:
    llm = ChatOpenAI(model="gpt-4o-mini", temperature=0.7, streaming=True)

    # Try semantic retrieval first; fall back to provided artifacts
    context_artifacts = _semantic_context(message) or _recency_context(artifacts)
    context = _build_context(context_artifacts)

    source_refs = ""
    if context_artifacts and context_artifacts[0].get("similarity") is not None:
        refs = [f"'{a['label']}'" for a in context_artifacts[:5]]
        source_refs = f"\n\nMost relevant sources: {', '.join(refs)}"

    system = f"""You are Aladin, a research intelligence assistant.
You help the user reason over their research collection — artifacts they have gathered including links, notes, documents, social media posts, and recordings.

Here are the most relevant items from their collection for this query:
{context}{source_refs}

Answer questions about their research, help connect ideas, surface patterns, and suggest what to explore next.
Be concise and direct. Cite specific artifact labels when referencing content.
If the user asks about something not in their collection, say so clearly."""

    messages = [SystemMessage(content=system)]

    for msg in history[-10:]:
        if msg["role"] == "user":
            messages.append(HumanMessage(content=msg["content"]))
        elif msg["role"] == "assistant":
            messages.append(AIMessage(content=msg["content"]))

    messages.append(HumanMessage(content=message))

    for chunk in llm.stream(messages):
        token = chunk.content
        if token:
            yield token
