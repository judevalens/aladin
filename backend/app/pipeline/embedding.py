"""
Generate and store OpenAI text embeddings for artifacts.
Uses text-embedding-3-small (1536 dims) stored in pgvector.
"""
from __future__ import annotations
import logging
from openai import OpenAI

logger = logging.getLogger(__name__)

_MAX_CHARS = 8000  # ~2000 tokens — stay well within model limit


def embed_text(text: str) -> list[float]:
    """Return a 1536-dim embedding vector for the given text."""
    client = OpenAI()
    text = text.strip()[:_MAX_CHARS]
    response = client.embeddings.create(
        model="text-embedding-3-small",
        input=text,
    )
    return response.data[0].embedding
