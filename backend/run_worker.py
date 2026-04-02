"""
Run the sync worker + pipeline worker.

    python run_worker.py

Reads DATABASE_URL and OPENAI_API_KEY from environment (or .env file).
"""
import logging
from dotenv import load_dotenv

load_dotenv()

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s — %(message)s",
)

from app.sync.queue import JobQueue
from app.sync.syncers import RedditSyncer, TwitterSyncer
from app.pipeline.worker import PipelineWorker
from app.pipeline.insight_worker import InsightWorker

# Start intelligence pipeline worker (enrichment → embedding → Neo4j)
pipeline_worker = PipelineWorker()
pipeline_worker.start()

# Start insight worker (runs generate_and_store on a schedule after enrichment)
insight_worker = InsightWorker()
insight_worker.start()

# Start sync job queue (scheduler + worker)
queue = (
    JobQueue
    .builder()
    .add(RedditSyncer())
    .add(TwitterSyncer())
    .build()
)

if __name__ == "__main__":
    queue.run()
