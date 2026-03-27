"""
Seed a Twitter source.

Usage:
    # Follow a user timeline
    python scripts/seed_twitter_source.py --mode user --username sama

    # Track a search query
    python scripts/seed_twitter_source.py --mode search --query "AI agents"

Requires:
    - A Knowledge Graph already seeded (uses the first one found)
    - A User already seeded
    - TWITTER_BEARER_TOKEN in .env
"""
import sys
import uuid
import argparse
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent))

from dotenv import load_dotenv
load_dotenv()

from app.db import get_session
from app.db.models import User, KnowledgeGraph, Source
from sqlalchemy import text


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--mode",     choices=["user", "search"], default="user")
    parser.add_argument("--username", help="Twitter username (mode=user)")
    parser.add_argument("--query",    help="Search query (mode=search)")
    parser.add_argument("--min-likes", type=int, default=5)
    parser.add_argument("--kg-id",    help="KnowledgeGraph UUID (defaults to first found)")
    args = parser.parse_args()

    if args.mode == "user" and not args.username:
        parser.error("--username required for mode=user")
    if args.mode == "search" and not args.query:
        parser.error("--query required for mode=search")

    with get_session() as session:
        # Pick user
        user = session.execute(text("SELECT id FROM users LIMIT 1")).fetchone()
        if not user:
            print("No users found. Run seed_reddit_source.py first or create a user.")
            sys.exit(1)

        # Pick or find KG
        if args.kg_id:
            kg = session.get(KnowledgeGraph, uuid.UUID(args.kg_id))
        else:
            kg = session.execute(
                text("SELECT id FROM knowledge_graphs LIMIT 1")
            ).fetchone()
            if kg:
                kg = session.get(KnowledgeGraph, kg.id)

        if not kg:
            print("No knowledge graph found. Seed one first.")
            sys.exit(1)

        # Build config
        if args.mode == "user":
            config = {
                "mode":      "user",
                "username":  args.username,
                "min_likes": args.min_likes,
                "max_results": 100,
            }
            name = f"@{args.username}"
        else:
            config = {
                "mode":        "search",
                "query":       args.query,
                "min_likes":   args.min_likes,
                "max_results": 100,
            }
            name = f"X: {args.query[:50]}"

        # Check for existing source
        existing = session.execute(text("""
            SELECT id FROM sources
            WHERE type = 'twitter' AND config->>'mode' = :mode
              AND (
                (:mode = 'user'   AND config->>'username' = :identifier) OR
                (:mode = 'search' AND config->>'query'    = :identifier)
              )
        """), {
            "mode":       args.mode,
            "identifier": args.username if args.mode == "user" else args.query,
        }).fetchone()

        if existing:
            print(f"Source already exists: {existing.id}")
            sys.exit(0)

        source = Source(
            id=uuid.uuid4(),
            user_id=user.id,
            kg_id=kg.id,
            name=name,
            type="twitter",
            sync_mode="poll",
            sync_state="active",
            config=config,
        )
        session.add(source)
        session.commit()

        print(f"Created Twitter source '{name}' (id={source.id})")
        print(f"  mode={args.mode}, min_likes={args.min_likes}")
        print(f"  KG: {kg.name} ({kg.id})")
        print()
        print("Make sure TWITTER_BEARER_TOKEN is set in .env, then run:")
        print("  poetry run python run_worker.py")


if __name__ == "__main__":
    main()
