"""
Seed r/AI_Agents as a Reddit source for the default user.

    poetry run python scripts/seed_reddit_source.py
"""
import uuid
import sys
from pathlib import Path
from datetime import datetime, timezone

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from dotenv import load_dotenv
load_dotenv()

from app.db import get_session
from app.db.models import User, KnowledgeGraph, Source
from app.db.seed import DEFAULT_USER_ID

KG_ID     = uuid.UUID("00000000-0000-0000-0000-000000000010")
SOURCE_ID = uuid.UUID("00000000-0000-0000-0000-000000000020")


def run():
    with get_session() as session:
        # Ensure default user exists
        user = session.get(User, DEFAULT_USER_ID)
        if not user:
            print("Default user not found — run the app first to seed the user")
            return

        # Create KG if not exists
        kg = session.get(KnowledgeGraph, KG_ID)
        if not kg:
            kg = KnowledgeGraph(
                id=KG_ID,
                user_id=DEFAULT_USER_ID,
                name="AI Research",
                description="Knowledge graph for AI agents and research",
                pipeline_autonomy="suggest",
            )
            session.add(kg)
            print("[seed] created KG: AI Research")

        # Create source if not exists
        source = session.get(Source, SOURCE_ID)
        if not source:
            source = Source(
                id=SOURCE_ID,
                user_id=DEFAULT_USER_ID,
                kg_id=KG_ID,
                name="r/AI_Agents",
                type="reddit",
                sync_mode="poll",
                sync_state="active",
                config={
                    "subreddit":        "AI_Agents",
                    "min_score":        5,
                    "include_comments": False,
                    "after":            None,
                    "last_seen_id":     None,
                    "sort":             "new",
                },
                auto_promote_threshold=0.85,
                suggest_threshold=0.60,
                next_sync_at=datetime.now(timezone.utc),  # due immediately
            )
            session.add(source)
            print("[seed] created source: r/AI_Agents")
        else:
            print("[seed] source already exists, skipping")

        session.commit()
        print(f"[seed] done — source_id: {SOURCE_ID}, kg_id: {KG_ID}")


if __name__ == "__main__":
    run()
