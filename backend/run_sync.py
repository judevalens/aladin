"""
Run one immediate sync cycle for a specific source type.

    python run_sync.py reddit
    python run_sync.py twitter

Reads DATABASE_URL and OPENAI_API_KEY from environment (or .env file).
"""
import sys
import logging
from dotenv import load_dotenv

load_dotenv()

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s — %(message)s",
)

from sqlalchemy import text
from app.sync.syncers import RedditSyncer, TwitterSyncer
from app.sync.queue import JobQueue
from app.db import get_session

SYNCERS = {
    "reddit":  RedditSyncer,
    "twitter": TwitterSyncer,
}

if __name__ == "__main__":
    if len(sys.argv) < 2 or sys.argv[1] not in SYNCERS:
        print(f"Usage: python run_sync.py [{' | '.join(SYNCERS)}]")
        sys.exit(1)

    source_type = sys.argv[1]

    # Reset next_sync_at so the scheduler picks up all sources of this type immediately
    with get_session() as session:
        result = session.execute(text("""
            UPDATE sources
            SET next_sync_at = NULL
            WHERE type = :type AND sync_state = 'active'
            RETURNING name
        """), {"type": source_type})
        names = [row.name for row in result.fetchall()]
        session.commit()

    if not names:
        print(f"No active sources found for type '{source_type}'")
        sys.exit(0)

    print(f"Triggering sync for: {', '.join(names)}")

    syncer_cls = SYNCERS[source_type]
    queue = (
        JobQueue
        .builder()
        .add(syncer_cls())
        .build()
    )
    queue.run()
