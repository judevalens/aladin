"""
Run the sync worker + scheduler.

    python run_worker.py

Reads DATABASE_URL from environment (or .env file).
"""
import logging
from dotenv import load_dotenv

load_dotenv()

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s — %(message)s",
)

from app.sync.queue import JobQueue
from app.sync.syncers import RedditSyncer, TwitterSyncer, InsightSyncer

queue = (
    JobQueue
    .builder()
    .add(RedditSyncer())
    .add(TwitterSyncer())
    .add(InsightSyncer())
    .build()
)

if __name__ == "__main__":
    queue.run()
