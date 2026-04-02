"""
Insight generation — finds non-obvious patterns across enriched artifacts.

Insight types:
  - bridge:       Entity appears across 2+ different source types (3+ total mentions)
  - convergence:  5+ artifacts share a topic in the last 7 days
  - trend:        Topic velocity is accelerating (more hits this week vs last)
"""
from __future__ import annotations
import logging
from collections import defaultdict
from datetime import datetime, timezone, timedelta

from sqlalchemy import text
from langchain_openai import ChatOpenAI
from langchain_core.messages import SystemMessage, HumanMessage

from app.db import get_session

logger = logging.getLogger(__name__)


# ── Helpers ───────────────────────────────────────────────────────────────────

def _llm_describe(prompt: str) -> str:
    """Generate a short natural language insight description."""
    try:
        llm = ChatOpenAI(model="gpt-4o-mini", temperature=0.4, max_tokens=200)
        msgs = [
            SystemMessage(content="You are a research intelligence assistant. Write concise, insight-oriented summaries (1-2 sentences). Be specific. No hedging."),
            HumanMessage(content=prompt),
        ]
        return llm.invoke(msgs).content.strip()
    except Exception as e:
        logger.warning("LLM describe failed: %s", e)
        return ""


# ── Generators ────────────────────────────────────────────────────────────────

def find_bridge_insights(kg_id: str) -> list[dict]:
    """
    Bridge: entity mentioned across 2+ distinct source types with 3+ total mentions.
    Surfaces hidden connections the user might not notice.
    """
    insights = []

    with get_session() as session:
        rows = session.execute(text("""
            WITH entity_sources AS (
                SELECT
                    jsonb_array_elements_text(a.enrichment->'entities') AS entity,
                    s.type AS source_type,
                    a.id   AS artifact_id,
                    a.label
                FROM artifacts a
                JOIN sources s ON s.id = a.source_id
                JOIN knowledge_graphs kg ON kg.id = s.kg_id
                WHERE kg.id = CAST(:kg_id AS uuid)
                  AND a.enrichment IS NOT NULL
                  AND a.status NOT IN ('superseded', 'dismissed')
            )
            SELECT
                entity,
                COUNT(DISTINCT source_type)         AS source_count,
                array_agg(DISTINCT source_type)     AS source_types,
                array_agg(DISTINCT artifact_id)     AS artifact_ids,
                COUNT(*)                             AS mention_count
            FROM entity_sources
            GROUP BY entity
            HAVING COUNT(DISTINCT source_type) >= 2
               AND COUNT(*) >= 3
            ORDER BY source_count DESC, mention_count DESC
            LIMIT 20
        """), {"kg_id": kg_id}).fetchall()

    for row in rows:
        sources = ", ".join(row.source_types[:4])
        prompt = (
            f"The concept/entity '{row.entity}' appears in {row.source_count} different "
            f"source types ({sources}) across {row.mention_count} artifacts in a research collection. "
            f"Write a brief insight title (max 12 words) and one-sentence explanation of why this cross-source "
            f"presence is significant. Format: TITLE | BODY"
        )
        description = _llm_describe(prompt)
        title, _, body = description.partition(" | ")
        if not body:
            title = f"'{row.entity}' bridges {sources}"
            body  = f"Mentioned across {row.source_count} source types — a signal of broad relevance."

        insights.append({
            "type":         "bridge",
            "title":        title.strip(),
            "body":         body.strip(),
            "entity":       row.entity,
            "artifact_ids": list(row.artifact_ids)[:20],
            "confidence":   min(0.6 + row.source_count * 0.1, 0.95),
        })

    return insights


def find_convergence_insights(kg_id: str) -> list[dict]:
    """
    Convergence: 5+ artifacts share a topic in the last 7 days.
    Signals an emerging area of focus.
    """
    since = datetime.now(timezone.utc) - timedelta(days=7)
    insights = []

    with get_session() as session:
        rows = session.execute(text("""
            WITH topic_artifacts AS (
                SELECT
                    jsonb_array_elements_text(a.enrichment->'topics') AS topic,
                    a.id   AS artifact_id,
                    a.label,
                    a.created_at
                FROM artifacts a
                JOIN sources s ON s.id = a.source_id
                JOIN knowledge_graphs kg ON kg.id = s.kg_id
                WHERE kg.id = CAST(:kg_id AS uuid)
                  AND a.enrichment IS NOT NULL
                  AND a.status NOT IN ('superseded', 'dismissed')
                  AND a.created_at >= :since
            )
            SELECT
                topic,
                COUNT(*)                         AS artifact_count,
                array_agg(DISTINCT artifact_id)  AS artifact_ids
            FROM topic_artifacts
            GROUP BY topic
            HAVING COUNT(*) >= 5
            ORDER BY artifact_count DESC
            LIMIT 15
        """), {"kg_id": kg_id, "since": since}).fetchall()

    for row in rows:
        prompt = (
            f"Topic '{row.topic}' appeared in {row.artifact_count} research artifacts in the last 7 days. "
            f"Write a convergence insight: title (max 10 words) and one-sentence explanation. "
            f"Format: TITLE | BODY"
        )
        description = _llm_describe(prompt)
        title, _, body = description.partition(" | ")
        if not body:
            title = f"Surge in '{row.topic}' activity"
            body  = f"{row.artifact_count} artifacts on this topic in the last 7 days — something is happening."

        insights.append({
            "type":         "convergence",
            "title":        title.strip(),
            "body":         body.strip(),
            "topic":        row.topic,
            "artifact_ids": list(row.artifact_ids)[:20],
            "confidence":   min(0.5 + row.artifact_count * 0.04, 0.9),
        })

    return insights


def find_trend_insights(kg_id: str) -> list[dict]:
    """
    Trend: topic velocity is higher this week vs last week.
    """
    now       = datetime.now(timezone.utc)
    this_week = now - timedelta(days=7)
    last_week = now - timedelta(days=14)
    insights  = []

    with get_session() as session:
        rows = session.execute(text("""
            WITH weekly AS (
                SELECT
                    jsonb_array_elements_text(a.enrichment->'topics') AS topic,
                    CASE WHEN a.created_at >= :this_week THEN 1 ELSE 0 END AS this_week,
                    CASE WHEN a.created_at < :this_week AND a.created_at >= :last_week THEN 1 ELSE 0 END AS last_week
                FROM artifacts a
                JOIN sources s ON s.id = a.source_id
                JOIN knowledge_graphs kg ON kg.id = s.kg_id
                WHERE kg.id = CAST(:kg_id AS uuid)
                  AND a.enrichment IS NOT NULL
                  AND a.status NOT IN ('superseded', 'dismissed')
                  AND a.created_at >= :last_week
            )
            SELECT
                topic,
                SUM(this_week) AS this_week_count,
                SUM(last_week) AS last_week_count
            FROM weekly
            GROUP BY topic
            HAVING SUM(this_week) >= 3
               AND SUM(this_week) > SUM(last_week) * 1.5
            ORDER BY (SUM(this_week)::float / GREATEST(SUM(last_week), 1)) DESC
            LIMIT 10
        """), {"kg_id": kg_id, "this_week": this_week, "last_week": last_week}).fetchall()

    for row in rows:
        ratio = row.this_week_count / max(row.last_week_count, 1)
        insights.append({
            "type":         "trend",
            "title":        f"'{row.topic}' gaining momentum",
            "body":         f"{row.this_week_count} mentions this week vs {row.last_week_count} last week ({ratio:.1f}x increase).",
            "topic":        row.topic,
            "artifact_ids": [],
            "confidence":   min(0.5 + ratio * 0.05, 0.85),
        })

    return insights


def generate_and_store(kg_id: str, snapshot_id: str | None = None) -> int:
    """
    Run all insight generators and persist new insights.
    Returns count of new insights stored.
    """
    all_insights = []
    try:
        all_insights += find_bridge_insights(kg_id)
    except Exception:
        logger.exception("Bridge insight generation failed")
    try:
        all_insights += find_convergence_insights(kg_id)
    except Exception:
        logger.exception("Convergence insight generation failed")
    try:
        all_insights += find_trend_insights(kg_id)
    except Exception:
        logger.exception("Trend insight generation failed")

    if not all_insights:
        return 0

    import json
    stored = 0
    with get_session() as session:
        for ins in all_insights:
            # Deduplicate: skip if same type+entity/topic exists in last 3 days
            dup_key  = ins.get("entity") or ins.get("topic") or ins["title"]
            existing = session.execute(text("""
                SELECT id FROM insights
                WHERE kg_id = CAST(:kg_id AS uuid)
                  AND type  = :type
                  AND (entity = :key OR topic = :key OR title = :title)
                  AND created_at >= now() - interval '3 days'
                LIMIT 1
            """), {
                "kg_id": kg_id,
                "type":  ins["type"],
                "key":   dup_key,
                "title": ins["title"],
            }).fetchone()

            if existing:
                continue

            session.execute(text("""
                INSERT INTO insights
                    (kg_id, type, title, body, artifact_ids, entity, topic, confidence, user_status)
                VALUES
                    (CAST(:kg_id AS uuid), :type, :title, :body,
                     CAST(:artifact_ids AS jsonb), :entity, :topic, :confidence, 'pending')
            """), {
                "kg_id":        kg_id,
                "type":         ins["type"],
                "title":        ins["title"],
                "body":         ins["body"],
                "artifact_ids": json.dumps(ins["artifact_ids"]),
                "entity":       ins.get("entity"),
                "topic":        ins.get("topic"),
                "confidence":   ins["confidence"],
            })
            stored += 1

        session.commit()

    logger.info("Stored %d new insights for kg %s", stored, kg_id)
    return stored
